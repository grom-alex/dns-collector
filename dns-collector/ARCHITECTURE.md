# Архитектура dns-collector

Документ описывает **текущее** состояние сервиса: как устроен приём запросов, резолвинг, хранение и очистка данных. Планы по изменению схемы и снижению нагрузки на диск вынесены в [../docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md).

## 1. Место в системе

~~~text
┌──────────────┐   DNS-запросы    ┌──────────────────┐
│  Клиенты     │ ───────────────▶ │ Unbound (pfSense)│
└──────────────┘                  └────────┬─────────┘
                                           │ Python-модуль
                                           │ UDP JSON :5353
                                           ▼
                                  ┌──────────────────┐
                                  │  dns-collector   │
                                  │  UDP + resolver  │
                                  │  cleanup + metrics│
                                  └────────┬─────────┘
                                           │ SQL
                                           ▼
                                  ┌──────────────────┐
                                  │   PostgreSQL 16  │
                                  └────────┬─────────┘
                                           │ SQL (read)
                                           ▼
                                  ┌──────────────────┐
                                  │     web-api      │
                                  │ Gin + Vue + Excel│
                                  └────────┬─────────┘
                                           │ HTTP :8080
                                           ▼
                                  ┌──────────────────┐
                                  │   Браузер / API  │
                                  └──────────────────┘
~~~

- `dns-collector` — приём UDP, запись в PostgreSQL, резолвинг, retention.
- `web-api` — чтение данных PostgreSQL, REST API и веб-интерфейс; при старте применяет миграции (DDL).
- PostgreSQL — единственное общее хранилище; файловых БД больше нет.
- Unbound-модуль — [../unbound/python_dns_forwarder.py](../unbound/python_dns_forwarder.py).

## 2. Компоненты

| Компонент | Файл | Ответственность |
|---|---|---|
| Точка входа | `cmd/dns-collector/main.go` | конфиг, БД, миграции, запуск/остановка сервисов |
| UDP-сервер | `internal/server/udp.go` | приём JSON, валидация, запись статистики и домена |
| Резолвер | `internal/resolver/resolver.go` | выборка доменов, DNS-lookup, upsert IP |
| Слой БД | `internal/database/database.go` | SQL, upsert, выборки, очистка |
| Миграции | `internal/database/migrations.go` + `migrations/*.sql` | схема и версии |
| Retention | `internal/cleanup/cleanup.go` | удаление старых статистик/IP/доменов |
| Метрики | `internal/metrics/*` | Prometheus, InfluxDB, сбор размеров БД |
| Конфиг | `internal/config/config.go` | YAML, env-overrides, валидация |

## 3. Приём UDP-запросов

### 3.1. Формат

~~~json
{
  "client_ip": "192.168.0.10",
  "domain": "google.com",
  "qtype": "AAAA",
  "rtype": "cache"
}
~~~

- `domain` обязателен; пустой отбрасывается.
- `client_ip` при отсутствии заменяется на `unknown`.
- `rtype` при отсутствии заменяется на `unknown`.
- `qtype` принимается, но пока не используется.

### 3.2. Обработка

1. `ReadFromUDP` читает датаграмму в буфер 4096 байт.
2. Данные копируются в новый срез (буфер переиспользуется).
3. На каждый пакет запускается горутина `handleMessage`.
4. `trimInvalidJSONSuffix` отрезает мусор после последней `}` (костыль под реальные пакеты Unbound).
5. `json.Unmarshal`; при ошибке — лог и метрика `invalid`.
6. `InsertDomainStat` — запись в `domain_stat`.
7. `InsertOrGetDomain` — `INSERT ... ON CONFLICT DO NOTHING RETURNING`; если строка уже есть, отдельный `SELECT`.
8. `UpdateDomainLastSeen` — `UPDATE domain SET last_seen = now()`.
9. Метрики: `valid`/`invalid`, `rtype`, время обработки, новые домены.

Ключевая особенность: **на каждый пакет выполняется 3–4 SQL-запроса**, а горутины не ограничены пулом. Это главный источник нагрузки на PostgreSQL и диск; подробный разбор — в [плане](../docs/OPTIMIZATION_PLAN.md#21-карта-горячего-пути).

### 3.3. Обработка ошибок

- `defer recover()` в `handleMessage` защищает процесс от паники при неожиданном JSON.
- Ошибки БД логируются и не прерывают приём.
- При ошибке `InsertOrGetDomain` запись статистики уже выполнена, а домен не обновлён — это допустимо, но приводит к рассинхронизации при сбоях БД.

## 4. Резолвер

### 4.1. Запуск

- При старте создаётся `net.Resolver` с `PreferGo: true` и таймаутом из конфига.
- Первый батч запускается сразу, далее — по тикеру `interval_seconds`.
- На каждый тик запускается новая горутина `runResolution`; защиты от наложения батчей нет.

### 4.2. Выборка доменов

`GetDomainsToResolve(limit, cyclicMode, cooldownMins)`:

~~~sql
-- cyclic_resolv: true
SELECT ... FROM domain
WHERE resolv_count < max_resolv
   OR (resolv_count >= max_resolv AND last_resolv_time < $1)
ORDER BY last_resolv_time ASC
LIMIT $2;
~~~

- Размер батча: `workers × 10`.
- В циклическом режиме сброс счётчика происходит при `resolv_count >= max_resolv - 1` в значение `⌊max_resolv × 2/3⌋`, поэтому домен **никогда не достигает** `max_resolv`. Ветка `resolv_count >= max_resolv` и cooldown на практике недостижимы — это отмечено и в коде, и в CHANGELOG.
- Следствие: все домены постоянно остаются кандидатами на резолв, пока работает тикер.

### 4.3. Резолвинг

Для каждого домена:

1. Один контекст с таймаутом `timeout_seconds` используется **и для IPv4, и для IPv6**.
2. `LookupIP(ctx, "ip4", domain)` и `LookupIP(ctx, "ip6", domain)`.
3. Для каждого адреса — `InsertOrUpdateIP`:
   ~~~sql
   INSERT INTO ip (domain_id, ip, type, time)
   VALUES ($1,$2,$3,$4)
   ON CONFLICT (domain_id, ip) DO UPDATE
   SET time = $5, type = $6;
   ~~~
   Даже если IP не изменился, строка переписывается, а `time` и `type` индексированы — HOT невозможен.
4. `UpdateDomainResolvStats` — инкремент `resolv_count` и `last_resolv_time` (в циклическом режиме — сброс на 2/3).
5. Логи на каждый домен и каждый IP.

Если IPv4-запрос израсходовал почти весь таймаут, IPv6-запрос почти гарантированно не успевает. Резолвинг идёт через системный резолвер (`/etc/resolv.conf`); отдельной настройки upstream DNS нет.

## 5. Retention и очистка

`internal/cleanup/cleanup.go` запускается сразу при старте и далее раз в `cleanup_interval_hours`.

| Что | Параметр | Логика |
|---|---|---|
| Статистика | `stats_days` (по умолчанию 30, максимум 365) | `DELETE FROM domain_stat WHERE timestamp < cutoff` |
| IP-адреса | `ip_ttl_days` (по умолчанию 3, максимум 90) | `DELETE FROM ip WHERE time < cutoff AND domain_id IN (SELECT id FROM domain WHERE last_seen >= cutoff)` — только у активных доменов |
| Домены | `domain_ttl_days` (0 = выключено, максимум 365) | удаляются домены с `last_seen IS NOT NULL` и старше cutoff; IP удаляются в той же транзакции |

Особенности текущей реализации:

- `ip_ttl_days = 0` в коде превращается в 3 (см. `config.go`); «отключить» очистку IP значением 0 нельзя. Это исправляется в [плане](../docs/OPTIMIZATION_PLAN.md#этап-7-web-api-и-экспорт-2-дня).
- Домены с `last_seen IS NULL` (никогда не запрашивались клиентами) не удаляются по TTL.
- `DELETE` по `domain_stat` — массовая операция; при больших объёмах она создаёт dead tuples и bloat. Переход на партиции — в [плане](../docs/OPTIMIZATION_PLAN.md#этап-3-партиционирование-domain_stat-3-дня).

## 6. Схема БД

### 6.1. Таблицы

~~~sql
CREATE TABLE domain (
    id SERIAL PRIMARY KEY,
    domain TEXT NOT NULL UNIQUE,
    time_insert TIMESTAMP NOT NULL,
    resolv_count INTEGER NOT NULL DEFAULT 0,
    max_resolv INTEGER NOT NULL,
    last_resolv_time TIMESTAMP NOT NULL,
    last_seen TIMESTAMP
);

CREATE TABLE ip (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domain(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    type TEXT NOT NULL,
    time TIMESTAMP NOT NULL,
    UNIQUE (domain_id, ip)
);

CREATE TABLE domain_stat (
    id SERIAL PRIMARY KEY,
    domain TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);
~~~

### 6.2. Индексы

- `domain`: PK, `UNIQUE(domain)`, `(resolv_count, max_resolv)`, `(last_seen)`, `(last_resolv_time)`, `(time_insert)`, partial-индексы по `resolv_count`.
- `ip`: PK, `UNIQUE(domain_id, ip)`, `(domain_id)`, `(type)`, `(time)`, `(ip)`, `(domain_id, type)`.
- `domain_stat`: `(timestamp DESC)`, `(client_ip)`, `(domain)`, `(rtype)`, `(timestamp DESC, client_ip)`, `(timestamp DESC, domain)`.

Часть индексов дублирует друг друга или стоит на обновляемых столбцах: `idx_domain_name` дублирует уникальный индекс, `idx_ip_time` дублирует `idx_ip_time_cleanup`, а индексы по `resolv_count`, `last_resolv_time`, `last_seen`, `ip.time`, `ip.type` запрещают HOT. Их стоимость и план удаления — в [плане](../docs/OPTIMIZATION_PLAN.md#22-индексы-которые-убивают-hot).

### 6.3. Управление схемой

- `database.New()` вызывает `initSchema()`, который создаёт таблицы и часть индексов через `CREATE TABLE IF NOT EXISTS`.
- Затем `RunMigrations()` применяет встроенные миграции `000001–000004`.
- На свежей БД фактическая схема определяется `initSchema()` (например, `domain.domain` — `TEXT`, а не `VARCHAR(255)`), миграции лишь фиксируют версию.
- В web-api есть **своя копия** тех же миграций; какой сервис стартует первым, тот и создаёт схему.
- Миграция `000003` содержит `ALTER DATABASE dns_collector` и `jit = on` — жёсткая привязка к имени БД и спорная настройка.

## 7. Поток записи и нагрузка на диск

| Операция | Когда | Объём |
|---|---|---|
| `INSERT domain_stat` | каждый UDP-пакет | 1 строка |
| `INSERT/SELECT domain` | каждый пакет для нового домена; для известного — `INSERT ... DO NOTHING` + `SELECT` | 1–2 запроса |
| `UPDATE domain.last_seen` | каждый пакет | 1 строка + индекс |
| `INSERT ip ... DO UPDATE` | каждый IP при каждом резолве | до нескольких строк на домен |
| `UPDATE domain.resolv_count/last_resolv_time` | каждый домен при каждом резолве | 1 строка + индексы |
| `DELETE domain_stat` | раз в сутки | до сотен тысяч строк |
| `DELETE ip/domain` | раз в сутки | меньше |

Основные причины избыточной записи:

1. `ip.time` обновляется при каждом резолве, даже если IP тот же.
2. `domain.last_seen` обновляется на каждый пакет.
3. Индексы на `resolv_count`, `last_resolv_time`, `last_seen`, `ip.time`, `ip.type` запрещают HOT.
4. `domain_stat` — не партиционированная churn-таблица с шестью индексами.

Цифры, замеры и пошаговый план — в [docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md).

## 8. Метрики

При `metrics.enabled: true`:

- HTTP-сервер на `:9090`: `/metrics`, `/health`.
- Prometheus-метрики:
  - `dns_server_messages_received_total{status}`, `dns_server_domains_received_total{rtype}`, `dns_server_new_domains_total`, `dns_server_processing_duration_seconds`;
  - `dns_resolver_lookups_total{ip_version,status}`, `dns_resolver_lookup_duration_seconds{ip_version}`, `dns_resolver_domains_processed_total{status}`, `dns_resolver_batch_size`, `dns_resolver_active_workers`;
  - `dns_cleanup_*` — удалённые строки, длительность, количество запусков;
  - `dns_db_domains_total`, `dns_db_ips_total` — обновляются раз в 30 с.
- Go runtime и process-метрики.
- Опционально — push в InfluxDB 2 (интервал `interval_seconds`).

## 9. Конфигурация

| Секция | Ключи |
|---|---|
| `server` | `udp_port` |
| `database` | `host`, `port`, `user`, `password`, `database`, `ssl_mode` |
| `resolver` | `interval_seconds`, `max_resolv`, `timeout_seconds`, `workers`, `cyclic_resolv`, `resolv_cooldown_mins` |
| `logging` | `level` (в collector фактически не применяется) |
| `retention` | `stats_days`, `cleanup_interval_hours`, `ip_ttl_days`, `domain_ttl_days` |
| `metrics` | `enabled`, `port`, `path`, `influxdb` |

Переопределения из окружения: `POSTGRES_PASSWORD`, `POSTGRES_SSL_MODE`, `INFLUXDB_TOKEN`.

Валидация при старте: порт UDP, интервал резолвера, `max_resolv`, retention-границы (30/365 дней, 168 ч, 90 дней), порт и путь метрик.

## 10. Развёртывание

- Docker multi-stage: сборка на `golang:1.24-bullseye`, runtime — `debian:bullseye-slim`, non-root пользователь `dnscollector`.
- Конфиг монтируется в `/app/config/config.yaml` (только чтение).
- Миграции применяются при старте; при ошибке миграции процесс завершается.
- В корневом `docker-compose.yml` опубликован только `5353/udp`; PostgreSQL — `5432`; web-api — `8080`. Порт метрик 9090 в корневом compose не публикуется (в production — публикуется).
- Healthcheck контейнера collector отсутствует; web-api проверяется через `/health`.

## 11. Известные ограничения

- Нет аутентификации на UDP-приёмнике: любой, кто видит порт, может прислать произвольные `domain`/`client_ip`.
- Нет ограничения горутин и rate limit на UDP.
- Батчи резолвера могут накладываться; один контекст на IPv4+IPv6.
- `UPDATE` на каждый пакет/резолв и индексы на обновляемых столбцах → высокий износ диска.
- `domain_stat` не партиционирована, retention через массовый `DELETE`.
- `initSchema` и миграции дублируют друг друга; схема зависит от порядка запуска сервисов.
- `ALTER DATABASE dns_collector` в миграции 000003.
- Экспорт в web-api ограничен памятью контейнера; `IN (...)` может превысить лимит параметров PostgreSQL.
- `logging.level` в collector не используется.

Полный перечень и приоритеты — в [docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md).

## 12. Карта файлов

~~~text
dns-collector/
├── cmd/dns-collector/main.go       # запуск, graceful shutdown
├── internal/
│   ├── config/config.go            # YAML + env + валидация
│   ├── database/
│   │   ├── database.go             # SQL и upsert
│   │   ├── migrations.go           # golang-migrate + embed
│   │   └── migrations/*.sql
│   ├── server/udp.go               # UDP-приёмник
│   ├── resolver/resolver.go        # пул воркеров и DNS
│   ├── cleanup/cleanup.go          # retention
│   └── metrics/                    # Prometheus/InfluxDB/DB-метрики
├── config/config.yaml
├── Dockerfile
└── ...                             # тесты, документация, скрипты
~~~
