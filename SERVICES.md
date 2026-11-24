# Архитектура микросервисов DNS Collector

## Обзор

Система состоит из двух Go-сервисов и PostgreSQL:

| Сервис | Роль | Порт |
|---|---|---|
| `dns-collector` | приём UDP, запись в БД, резолвинг, retention, метрики | 5353/udp, 9090/tcp |
| `web-api` | REST API, Vue UI, Excel/pfSense-экспорт | 8080/tcp |
| PostgreSQL 16 | общее хранилище | 5432/tcp |

Оба сервиса — отдельные Go-модули (`dns-collector/go.mod`, `web-api/go.mod`) и отдельные Docker-образы. Общей библиотеки между ними нет: часть кода (миграции, метрики, структуры конфига) дублируется.

## Схема

~~~text
                 UDP JSON :5353
┌──────────────┐ ───────────────▶ ┌─────────────────────────┐
│ Unbound /    │                  │      dns-collector      │
│ pfSense      │                  │ UDP + resolver + cleanup│
└──────────────┘                  │ + Prometheus :9090      │
                                  └───────────┬─────────────┘
                                              │ SQL (read/write)
                                              ▼
                                  ┌─────────────────────────┐
                                  │      PostgreSQL 16      │
                                  │ domain / ip / domain_stat│
                                  └───────────┬─────────────┘
                                              │ SQL (read + migrations)
                                              ▼
                                  ┌─────────────────────────┐
                                  │         web-api         │
                                  │ Gin + Vue + Excel       │
                                  │ HTTP :8080              │
                                  └───────────┬─────────────┘
                                              │ HTTP
                                              ▼
                                  ┌─────────────────────────┐
                                  │ Браузер, API, pfSense   │
                                  └─────────────────────────┘
~~~

## dns-collector

### Назначение

Принимает DNS-события, пишет статистику и домены, периодически резолвит их в IPv4/IPv6, чистит устаревшие данные.

### Компоненты

- **UDP-сервер** — JSON-сообщения, валидация, запись.
- **Резолвер** — пул воркеров, `LookupIP` для IPv4/IPv6.
- **Cleanup** — retention статистики, IP и доменов.
- **Метрики** — Prometheus + опционально InfluxDB 2.
- **Миграции** — `golang-migrate` с embedded SQL.

Подробно: [dns-collector/ARCHITECTURE.md](dns-collector/ARCHITECTURE.md).

### Конфигурация

`dns-collector/config/config.yaml`:

~~~yaml
server:
  udp_port: 5353
  udp_workers: 16
  udp_queue_size: 4096

database:
  host: "postgres"
  port: 5432
  user: "dns_collector"
  password: ""          # POSTGRES_PASSWORD
  database: "dns_collector"
  ssl_mode: "disable"

resolver:
  interval_seconds: 300
  max_resolv: 10
  timeout_seconds: 5
  workers: 5
  cyclic_resolv: true
  resolv_cooldown_mins: 240
  ip_refresh_hours: 24

retention:
  stats_days: 30
  cleanup_interval_hours: 24
  ip_ttl_days: 3
  domain_ttl_days: 30

metrics:
  enabled: true
  port: 9090
  path: "/metrics"
~~~

### Запуск

~~~bash
# Docker
docker compose up -d dns-collector

# Нативно
cd dns-collector
go run ./cmd/dns-collector -config config/config.yaml
~~~

## web-api

### Назначение

Читает PostgreSQL, отдаёт REST API и SPA, формирует Excel и plain-text списки для pfSense.

### Компоненты

- **Gin HTTP-сервер** — `/api/*`, `/health`, `/metrics`, статика SPA.
- **Слой БД** — параметризованные запросы, regex-фильтры, bulk-выборка IP.
- **Excel-экспорт** — `excelize`, до 100 000 строк.
- **Export lists** — динамические эндпоинты из конфига.

### API

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/health` | health-check (не проверяет БД) |
| GET | `/api/stats` | статистика с фильтрами и пагинацией |
| GET | `/api/stats/export` | Excel со статистикой |
| GET | `/api/domains` | домены с фильтрами |
| GET | `/api/domains/export` | Excel: домены + IP |
| GET | `/api/domains/:id` | домен с IP |
| GET | `/metrics` | Prometheus (если `metrics.enabled`) |
| GET | `<endpoint>` из `export_lists` | plain-text список доменов/IP |
| GET | `<excluded_ips_endpoint>` | аудит исключённых shared IP |

Параметры `/api/stats`: `client_ips`, `subnet`, `date_from`, `date_to`, `sort_by`, `sort_order`, `limit`, `offset`.

Параметры `/api/domains`: `domain_regex` (PostgreSQL regex), `date_from`, `date_to`, `sort_by`, `sort_order`, `limit`, `offset`.

### Конфигурация

`web-api/config/config.yaml`:

~~~yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  host: "postgres"
  port: 5432
  user: "dns_collector"
  password: ""
  database: "dns_collector"
  ssl_mode: "disable"

cors:
  allowed_origins:
    - "http://localhost:8080"
    - "http://localhost:5173"
  allow_credentials: true

metrics:
  enabled: true
  path: "/metrics"

export_lists:
  - name: "Example"
    endpoint: "/export/example"
    domain_regex: "^example\\.com$"
    include_domains: true
~~~

Полное описание export lists: [web-api/EXPORT_LISTS.md](web-api/EXPORT_LISTS.md).

### Запуск

~~~bash
# Docker
docker compose up -d web-api

# Backend локально
cd web-api
go run ./cmd -config config/config.yaml

# Frontend dev-server
cd web-api/frontend
npm install
npm run dev
~~~

## Общая база данных

- **dns-collector**: read/write, создаёт схему и применяет миграции.
- **web-api**: read + применяет те же миграции при старте; данные не изменяет (кроме схемы).
- Оба сервиса ходят в одну БД; конфиги `database` должны совпадать.
- Миграции дублируются в обоих репозиториях и должны быть идентичны.

Таблицы:

- `domain` — домены и состояние резолвинга;
- `ip` — IP-адреса доменов;
- `domain_stat` — статистика запросов.

## Запуск всей системы

~~~bash
docker compose up -d
docker compose ps
docker compose logs -f
~~~

Порты:

- `5353/udp` — collector;
- `8080/tcp` — web-api/UI;
- `9090/tcp` — метрики collector (в production-compose);
- `5432/tcp` — PostgreSQL (лучше не публиковать).

## Мониторинг

### Логи

~~~bash
docker compose logs -f dns-collector
docker compose logs -f web-api
~~~

### Метрики

- Collector: `http://localhost:9090/metrics`
- web-api: `http://localhost:8080/metrics`

Основные группы:

- `dns_server_*` — UDP-приём;
- `dns_resolver_*` — резолвинг;
- `dns_cleanup_*` — retention;
- `dns_db_*` — размеры БД;
- `http_*`, `api_*` — web-api.

### Проверка данных

~~~bash
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT count(*) FROM domain;"

curl -s http://localhost:8080/health
curl -s "http://localhost:8080/api/stats?limit=5"
~~~

## Масштабирование

- **web-api** масштабируется горизонтально (read-only по данным): несколько инстансов за балансировщиком; миграции при старте должны быть идемпотентны.
- **dns-collector** — один инстанс: резолвер не координируется между репликами, несколько экземпляров начнут дублировать резолвинг и запись.
- Для роста приёма UDP можно разделить приём и резолвинг, но это отдельная архитектурная задача.
- Узкое место сейчас — запись в PostgreSQL; см. [docs/OPTIMIZATION_PLAN.md](docs/OPTIMIZATION_PLAN.md).

## Безопасность

- Аутентификации нет ни в API, ни в UI; ставьте за reverse proxy с auth.
- UDP-приёмник не проверяет источник; ограничьте фаерволом.
- Смените дефолтные пароли, не публикуйте PostgreSQL.
- CORS настраивается в `web-api/config/config.yaml`.
- Collector работает non-root; web-api — от root (см. план).

## Бэкап

~~~bash
docker compose exec -T postgres \
  pg_dump -U dns_collector -d dns_collector -Fc > dns_$(date +%F).dump
~~~

Восстановление — в [dns-collector/DOCKER.md](dns-collector/DOCKER.md#восстановление).

## Документация

- [README.md](README.md)
- [SUMMARY.md](SUMMARY.md)
- [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md)
- [dns-collector/ARCHITECTURE.md](dns-collector/ARCHITECTURE.md)
- [dns-collector/QUICKSTART.md](dns-collector/QUICKSTART.md)
- [dns-collector/DOCKER.md](dns-collector/DOCKER.md)
- [web-api/README.md](web-api/README.md)
- [docs/OPTIMIZATION_PLAN.md](docs/OPTIMIZATION_PLAN.md)
