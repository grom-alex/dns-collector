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
| UDP-сервер | `internal/server/udp.go` | приём JSON, валидация, ограниченный пул воркеров |
| Event writer | `internal/writer/writer.go` | кэш доменов, батчинг `domain_stat`, троттлинг `last_seen` |
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
3. Пакет кладётся в ограниченную очередь `server.udp_queue_size`; её разбирает пул из `server.udp_workers` воркеров. При переполнении пакет отбрасывается и учитывается в `dns_server_messages_dropped_total{reason="udp_queue_full"}`.
4. `trimInvalidJSONSuffix` отрезает мусор после последней `}` (костыль под реальные пакеты Unbound).
5. `json.Unmarshal` и валидация; при ошибке — лог и метрика `invalid`.
6. `writer.Submit`:
   - domain ID ищется в LRU-кэше (`domain -> id`, TTL 24h);
   - при промахе — один `InsertOrGetDomain` (и запись в кэш);
   - строка статистики ставится в очередь и пишется батчем 500 строк / 1s;
   - `last_seen` агрегируется в памяти и сбрасывается батчем раз в 60s (не чаще 5 минут на домен).
7. Метрики: `valid`/`invalid`, `rtype`, время обработки, новые домены, dropped.

Ключевая особенность: **на каждый пакет больше нет 3–4 SQL-запросов**. В штатном режиме (cache hit) пакет не делает SQL вообще: он попадает в очередь статистики и в in-memory `last_seen`. SQL выполняется только на промахе кэша и при плановом flush. Подробный разбор — в [плане](../docs/OPTIMIZATION_PLAN.md#21-карта-горячего-пути).

### 3.3. Обработка ошибок

- `defer recover()` в `handleMessage` защищает процесс от паники при неожиданном JSON.
- Ошибки БД логируются и не прерывают приём.
- При переполнении очереди пакет отбрасывается; это сознательный backpressure вместо неограниченного роста памяти.
- Батч статистики при ошибке БД логируется и теряется; `last_seen` при ошибке возвращается в карту и повторяется на следующем flush.

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

1. IPv4 и IPv6 получают **отдельные контексты** с таймаутом `timeout_seconds`.
2. `LookupIP(ctx4, "ip4", domain)` и `LookupIP(ctx6, "ip6", domain)`.
3. Результат каждого успешного lookup передаётся в `UpsertIPs`:
   - новые адреса вставляются (`ON CONFLICT DO NOTHING`);
   - для вернувшихся адресов `time` обновляется не чаще `ip_refresh_hours` (по умолчанию 24h);
   - адреса, которые больше не возвращаются, не обновляются и удаляются штатным retention по TTL, а не немедленным DELETE (защита от round-robin/CDN churn).
4. `UpdateDomainResolvStats` — инкремент `resolv_count` и `last_resolv_time` (в циклическом режиме — сброс на 2/3).
5. Логи — только ошибки; логи на каждый домен/IP удалены.

Резолвинг идёт через системный резолвер (`/etc/resolv.conf`); отдельной настройки upstream DNS нет.

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
| `INSERT/SELECT domain` | только при промахе кэша `domain -> id` | 1–2 запроса на новый/редкий домен |
| `INSERT domain_stat` | батчем из очереди | 500 строк / 1s |
| `UPDATE domain.last_seen` | батчем раз в 60s | не чаще 5 минут на домен |
| `INSERT ... ON CONFLICT DO NOTHING` + условный `UPDATE ip.time` | пул адресов при резолве | новые + refresh раз в `ip_refresh_hours` |
| `UPDATE domain.resolv_count/last_resolv_time` | каждый домен при каждом резолве | 1 строка + индексы |
| `DELETE domain_stat` | раз в сутки | до сотен тысяч строк |
| `DELETE ip/domain` | раз в сутки | меньше |

Основные оставшиеся причины избыточной записи:

1. `domain.resolv_count` и `last_resolv_time` обновляются на каждый резолв, а индексы по ним запрещают HOT.
2. `domain_stat` — не партиционированная churn-таблица с шестью индексами: INSERT на каждый пакет и массовый DELETE по retention.
3. `ip.time` всё ещё обновляется раз в сутки на каждый вернувшийся IP; индексы по `time`/`type` препятствуют HOT.
4. `domain_stat` и `ip` остаются раздутыми; индексы занимают ~93% БД.

Что уже сделано в этапе 1: кэш доменов, батчинг статистики, троттлинг `last_seen`, upsert IP без перезаписи неизменных адресов и ограниченный пул воркеров. Следующие шаги — индексы, `fillfactor`, партиционирование и тюнинг PostgreSQL.

Цифры, замеры и пошаговый план — в [docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md).

## 8. Метрики

При `metrics.enabled: true`:

- HTTP-сервер на `:9090`: `/metrics`, `/health`.
- Prometheus-метрики:
  - `dns_server_messages_received_total{status}`, `dns_server_messages_dropped_total{reason}`, `dns_server_domains_received_total{rtype}`, `dns_server_new_domains_total`, `dns_server_processing_duration_seconds`;
  - `dns_resolver_lookups_total{ip_version,status}`, `dns_resolver_lookup_duration_seconds{ip_version}`, `dns_resolver_domains_processed_total{status}`, `dns_resolver_batch_size`, `dns_resolver_active_workers`;
  - `dns_cleanup_*` — удалённые строки, длительность, количество запусков;
  - `dns_db_domains_total`, `dns_db_ips_total` — обновляются раз в 30 с.
- Go runtime и process-метрики.
- Опционально — push в InfluxDB 2 (интервал `interval_seconds`).

## 9. Конфигурация

| Секция | Ключи |
|---|---|
| `server` | `udp_port`, `udp_workers`, `udp_queue_size` |
| `database` | `host`, `port`, `user`, `password`, `database`, `ssl_mode` |
| `resolver` | `interval_seconds`, `max_resolv`, `timeout_seconds`, `workers`, `cyclic_resolv`, `resolv_cooldown_mins`, `ip_refresh_hours` |
| `logging` | `level` (в collector фактически не применяется) |
| `retention` | `stats_days`, `cleanup_interval_hours`, `ip_ttl_days`, `domain_ttl_days` |
| `metrics` | `enabled`, `port`, `path`, `influxdb` |

Переопределения из окружения: `POSTGRES_PASSWORD`, `POSTGRES_SSL_MODE`, `INFLUXDB_TOKEN`.

Валидация при старте: порт UDP, интервал резолвера, `max_resolv`, retention-границы (30/365 дней, 168 ч, 90 дней), порт и путь метрик.

## 10. Развёртывание

- Docker multi-stage: сборка на `golang:1.24-bookworm`, runtime — `debian:bookworm-slim`, non-root пользователь `dnscollector`.
- Конфиг монтируется в `/app/config/config.yaml` (только чтение).
- Миграции применяются при старте; при ошибке миграции процесс завершается.
- В корневом `docker-compose.yml` опубликован только `5353/udp`; PostgreSQL — `5432`; web-api — `8080`. Порт метрик 9090 в корневом compose не публикуется (в production — публикуется).
- Healthcheck контейнера collector отсутствует; web-api проверяется через `/health`.

## 11. Известные ограничения

- Нет аутентификации на UDP-приёмнике: любой, кто видит порт, может прислать произвольные `domain`/`client_ip`.
- UDP-воркеры и очередь ограничены, но при переполнении пакеты дропаются (метрика `dns_server_messages_dropped_total`).
- Батчи резолвера могут накладываться; контексты IPv4/IPv6 раздельные.
- Остаются `UPDATE domain` на каждый резолв и массовые `DELETE domain_stat`; индексы на обновляемых столбцах всё ещё препятствуют HOT.
- Батч статистики при ошибке БД логируется и теряется (счётчика потерь пока нет).
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
