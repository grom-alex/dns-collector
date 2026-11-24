# Docker-развёртывание dns-collector

## Требования

- Docker 24+ и Docker Compose v2 (`docker compose ...`).
- Свободный UDP-порт 5353.
- PostgreSQL 16 — в compose или внешний.

## Сервисы в корневом compose

| Сервис | Образ | Порты | Данные |
|---|---|---|---|
| `postgres` | `postgres:16-alpine` | 5432 | volume `postgres_data` |
| `dns-collector` | `registry.gromas.ru/apps/dns-collector/dns-collector:latest` | 5353/udp | конфиг `./dns-collector/config` |
| `web-api` | `registry.gromas.ru/apps/dns-collector/web-api:latest` | 8080 | конфиг `./web-api/config` |

Корневой compose использует `pull_policy: always`. Для локальной сборки:

~~~bash
docker compose build dns-collector web-api
docker compose up -d
~~~

## Запуск

~~~bash
# из корня репозитория
docker compose up -d
docker compose ps
docker compose logs -f dns-collector
~~~

Остановка:

~~~bash
docker compose down          # данные PostgreSQL сохраняются
docker compose down -v       # удалить volume с данными
~~~

## Запуск только collector

Если PostgreSQL уже есть:

~~~bash
docker run -d \
  --name dns-collector \
  --restart unless-stopped \
  -p 5353:5353/udp \
  -p 9090:9090 \
  -v "$(pwd)/dns-collector/config:/app/config:ro" \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -e POSTGRES_SSL_MODE=disable \
  registry.gromas.ru/apps/dns-collector/dns-collector:latest
~~~

## Конфигурация

Конфиг монтируется только для чтения:

~~~yaml
volumes:
  - ./dns-collector/config:/app/config:ro
~~~

Внутри контейнера используется `/app/config/config.yaml`. Пароль БД и токен InfluxDB передаются через окружение:

- `POSTGRES_PASSWORD`
- `POSTGRES_SSL_MODE`
- `INFLUXDB_TOKEN`

После изменения конфига:

~~~bash
docker compose restart dns-collector
~~~

## Метрики

Collector слушает `:9090`:

- `http://localhost:9090/metrics`
- `http://localhost:9090/health`

В корневом compose порт 9090 **не опубликован**; добавьте:

~~~yaml
ports:
  - "9090:9090"
~~~

или используйте production-compose, где он уже опубликован.

## Бэкап PostgreSQL

~~~bash
# Дамп в файл на хосте
docker compose exec -T postgres \
  pg_dump -U dns_collector -d dns_collector -Fc \
  > "backup_$(date +%F).dump"

# Проверка размера
ls -lh backup_*.dump
~~~

Ежедневный cron:

~~~bash
0 2 * * * cd /opt/dns-collector && docker compose exec -T postgres \
  pg_dump -U dns_collector -d dns_collector -Fc \
  > "/backups/dns_$(date +%F).dump"
~~~

Ротация:

~~~bash
find /backups -name 'dns_*.dump' -mtime +30 -delete
~~~

## Восстановление

~~~bash
# Остановить пишущие сервисы
docker compose stop dns-collector web-api

# Пересоздать БД (осторожно: удалит данные)
docker compose exec -T postgres psql -U dns_collector -d postgres -c "
DROP DATABASE IF EXISTS dns_collector;"
docker compose exec -T postgres psql -U dns_collector -d postgres -c "
CREATE DATABASE dns_collector OWNER dns_collector;"

# Восстановить дамп
docker compose exec -T postgres \
  pg_restore -U dns_collector -d dns_collector --no-owner < backup_2026-11-22.dump

# Запустить обратно
docker compose start dns-collector web-api
~~~

## Обновление

~~~bash
docker compose pull
docker compose up -d
docker compose logs -f dns-collector
~~~

Миграции применяются при старте. Перед обновлением сделайте бэкап. Если миграция неуспешна — верните предыдущий тег образа и восстановите дамп.

## Логи и диагностика

~~~bash
docker compose logs --tail=200 dns-collector
docker compose logs -f web-api
docker stats --no-stream
docker inspect dns-collector-postgres --format '{{json .State.Health}}'
~~~

Логи ограничены в compose: `max-size: 50m`, `max-file: 5` (production) и `10m/3` (dev).

## Проверка БД

~~~bash
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT count(*) AS domains FROM domain;"

docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT count(*) AS requests FROM domain_stat;"

docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT domain, count(*) FROM domain_stat
GROUP BY domain ORDER BY count(*) DESC LIMIT 10;"
~~~

## Troubleshooting

### Контейнер не запускается

~~~bash
docker compose logs dns-collector
docker compose config
~~~

Частые причины: нет доступа к PostgreSQL, неверный пароль, занят UDP-порт, миграция упала.

### `failed to ping database`

- В compose host должен быть `postgres`, а не `localhost`.
- Проверьте `POSTGRES_PASSWORD` и `POSTGRES_SSL_MODE`.
- Проверьте healthcheck PostgreSQL.

### Порт занят

~~~bash
sudo ss -ulnp | grep 5353
~~~

Измените внешний порт в compose:

~~~yaml
ports:
  - "5454:5353/udp"
~~~

и `server.udp_port` в конфиге, если меняете внутренний.

### Права на конфиг

Конфиг монтируется read-only; убедитесь, что файл читается пользователем `dnscollector` (UID 1000) внутри контейнера.

## Ограничения ресурсов

В compose уже заданы `deploy.resources`. Для VM с 16 ГБ:

- PostgreSQL: 2 CPU / 4–8 ГБ RAM;
- collector: 1–2 CPU / 512 МБ;
- web-api: 1 CPU / 512 МБ.

## Безопасность

- Не публикуйте PostgreSQL (5432) наружу без необходимости.
- Смените дефолтный пароль `dns_collector_pass`.
- web-api не имеет встроенной аутентификации: ставьте за reverse proxy с auth (см. план).
- UDP-порт 5353 ограничьте на фаерволе адресом pfSense/Unbound.
- Collector работает non-root; web-api в текущем Dockerfile — от root (исправление в плане).
