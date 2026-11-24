# DNS Collector — краткий обзор

## Назначение

Система собирает DNS-запросы из Unbound/pfSense, хранит статистику и справочник доменов в PostgreSQL, периодически резолвит домены в IPv4/IPv6 и предоставляет веб-интерфейс, REST API и экспорт блок-листов для pfSense.

Типовой сценарий:

1. Unbound на pfSense через Python-модуль отправляет JSON по UDP на `dns-collector:5353`.
2. Collector пишет статистику, ведёт домены и IP, чистит устаревшие данные.
3. web-api читает PostgreSQL, отдаёт Vue-дашборд, Excel и plain-text списки для firewall alias.
4. Опционально — метрики Prometheus/InfluxDB и алерты.

## Компоненты

| Компонент | Технологии | Роль |
|---|---|---|
| `dns-collector` | Go 1.24, PostgreSQL, `lib/pq`, `golang-migrate` | приём UDP, резолвинг, retention, метрики |
| `web-api` | Go 1.24, Gin, Vue 3, Vite, `excelize` | REST API, UI, Excel и pfSense-экспорт |
| PostgreSQL 16 | SQL | общее хранилище |
| Unbound-модуль | Python | отправка DNS-событий с pfSense |

## Поток данных

~~~text
Unbound/pfSense ──UDP JSON──▶ dns-collector ──SQL──▶ PostgreSQL
                                                   │
                                      web-api ─────┘──HTTP──▶ браузер / API
                                         │
                                         └──▶ Excel / pfSense lists
~~~

## Возможности

### Сбор и резолвинг

- UDP-приёмник JSON (`client_ip`, `domain`, `qtype`, `rtype`).
- Хранение статистики каждого запроса (`domain_stat`).
- Справочник доменов и IP-адресов (IPv4 + IPv6).
- Периодический резолвинг пулом воркеров.
- Циклический режим (`cyclic_resolv`) со сбросом счётчика на `⌊max_resolv × 2/3⌋`.
- Retention: статистика, TTL IP, TTL доменов по `last_seen`.
- Prometheus-метрики и опциональный push в InfluxDB 2.

### Web API и UI

- Статистика с фильтрами по клиенту, подсети (CIDR), датам, сортировкой и пагинацией.
- Домены с поиском по regex и просмотром IP.
- Экспорт статистики и доменов в Excel (до 100 000 строк).
- Динамические export-эндпоинты для pfSense:
  - фильтр по regex;
  - IPv4/IPv6;
  - исключение shared IP (CDN/облака);
  - дополнительные IP из threat-intel файла;
  - отдельный эндпоинт для аудита исключённых IP.

## Схема БД (сокращённо)

- `domain` — домен, счётчик резолвов, `max_resolv`, `last_resolv_time`, `last_seen`.
- `ip` — IP домена, тип (`ipv4`/`ipv6`), время; уникальность `(domain_id, ip)`.
- `domain_stat` — домен, клиент, `rtype`, время запроса.

Полная схема и индексы: [dns-collector/ARCHITECTURE.md](dns-collector/ARCHITECTURE.md#6-схема-бд).

## Развёртывание

~~~bash
docker compose up -d
~~~

- UI: `http://localhost:8080`
- UDP: `5353`
- PostgreSQL: `5432`
- Метрики collector: `9090` (в production-compose)

Production-конфигурация: [deploy/production](deploy/production/). Подробнее: [DEPLOYMENT.md](DEPLOYMENT.md).

## Документация

- [CONTRIBUTING.md](CONTRIBUTING.md) — порядок ветвления, PR, тестов, релиза и деплоя.
- [README.md](README.md) — основная документация.
- [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md) — быстрый старт с UI/API.
- [SERVICES.md](SERVICES.md) — архитектура микросервисов.
- [dns-collector/ARCHITECTURE.md](dns-collector/ARCHITECTURE.md) — внутреннее устройство collector.
- [web-api/README.md](web-api/README.md) — API и UI.
- [TESTING.md](TESTING.md) — тесты и покрытие.
- [docs/OPTIMIZATION_PLAN.md](docs/OPTIMIZATION_PLAN.md) — план устранения недостатков и оптимизации БД.
- [docs/DB_BASELINE.md](docs/DB_BASELINE.md) — опорный срез нагрузки на БД от 2026-11-23.
- [docs/DB_STATISTICS.md](docs/DB_STATISTICS.md) — pg_stat_statements: включение и первый срез.
- [docs/DB_MONITORING.md](docs/DB_MONITORING.md) — 24-часовой мониторинг PostgreSQL.
- [docs/DB_MONITORING_REPORT.md](docs/DB_MONITORING_REPORT.md) — результаты 24-часового профиля.

## Известные ограничения

- web-api без аутентификации.
- UDP-приёмник доверяет `client_ip`/`domain` из пакета.
- Высокая нагрузка на диск из-за UPDATE на каждый пакет/резолв и индексов на обновляемых столбцах.
- `domain_stat` не партиционирована; retention через массовый `DELETE`.
- Дублирование схемы (`initSchema` + миграции) и кода между сервисами.
- Часть тестов не покрывает реальные SQL/бизнес-пути.

Приоритеты и конкретные шаги — в [docs/OPTIMIZATION_PLAN.md](docs/OPTIMIZATION_PLAN.md).
