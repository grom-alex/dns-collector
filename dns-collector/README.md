# dns-collector

Сервис сбора DNS-запросов и периодического резолвинга доменов.

## Что делает

- Принимает по UDP JSON-сообщения о DNS-запросах от Unbound/pfSense или тестового клиента.
- Сохраняет статистику запросов в PostgreSQL (`domain_stat`).
- Ведёт справочник доменов (`domain`) и их IP-адресов (`ip`, IPv4 и IPv6).
- Периодически резолвит домены через системный DNS и обновляет IP.
- Чистит устаревшие статистику, IP и домены по retention-политике.
- Отдаёт метрики Prometheus и, опционально, отправляет их в InfluxDB.

Web-интерфейс и REST API — отдельный сервис [web-api](../web-api/).

## Быстрый старт

Из корня репозитория:

~~~bash
docker compose up -d
docker compose logs -f dns-collector
~~~

Проверка:

~~~bash
echo '{"client_ip":"192.168.1.10","domain":"example.com","qtype":"A","rtype":"dns"}' | nc -u -w1 localhost 5353
docker compose exec -T postgres psql -U dns_collector -d dns_collector \
  -c "SELECT domain, resolv_count, last_resolv_time FROM domain ORDER BY id DESC LIMIT 5;"
~~~

Подробнее: [QUICKSTART.md](QUICKSTART.md), [INSTALL.md](INSTALL.md), [DOCKER.md](DOCKER.md).

## Формат входящего сообщения

~~~json
{
  "client_ip": "192.168.0.10",
  "domain": "google.com",
  "qtype": "AAAA",
  "rtype": "cache"
}
~~~

| Поле | Обязательное | Назначение |
|---|---|---|
| `client_ip` | нет | IP клиента; при отсутствии подставляется `unknown` |
| `domain` | да | Доменное имя; пустое отбрасывается |
| `qtype` | нет | Тип запроса (A, AAAA, CNAME...), сейчас не используется |
| `rtype` | нет | Источник ответа: `cache`/`dns`/`reply`; при отсутствии — `unknown` |

## Конфигурация

Файл по умолчанию: `config/config.yaml`. Основные параметры:

~~~yaml
server:
  udp_port: 5353

database:
  host: "postgres"
  port: 5432
  user: "dns_collector"
  password: ""          # обычно через POSTGRES_PASSWORD
  database: "dns_collector"
  ssl_mode: "disable"   # или require

resolver:
  interval_seconds: 300
  max_resolv: 10
  timeout_seconds: 5
  workers: 5
  cyclic_resolv: true
  resolv_cooldown_mins: 240

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

Переменные окружения: `POSTGRES_PASSWORD`, `POSTGRES_SSL_MODE`, `INFLUXDB_TOKEN`.

Полное описание схемы и режимов резолвинга — в [ARCHITECTURE.md](ARCHITECTURE.md).

## Метрики

При `metrics.enabled: true` сервис поднимает HTTP на `:9090`:

- `/metrics` — Prometheus;
- `/health` — простой health-check.

В корневом `docker-compose.yml` порт 9090 не публикуется наружу; в production-конфигурации — публикуется. Подробности: [../METRICS.md](../METRICS.md).

## Тесты

~~~bash
cd dns-collector
go test ./internal/... ./cmd/...
go test -race ./internal/...
~~~

Покрытие и известные пробелы: [../TESTING.md](../TESTING.md).

## Документация

- [ARCHITECTURE.md](ARCHITECTURE.md) — компоненты, схема БД, потоки записи.
- [QUICKSTART.md](QUICKSTART.md) — запуск и проверка.
- [INSTALL.md](INSTALL.md) — нативная установка и systemd.
- [DOCKER.md](DOCKER.md) — Docker/Compose, бэкапы, обновление.
- [../SERVICES.md](../SERVICES.md) — взаимодействие с web-api.
- [../docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md) — план оптимизации БД и устранения недостатков.
