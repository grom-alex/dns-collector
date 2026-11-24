# Быстрый старт dns-collector

## 1. Docker Compose (рекомендуется)

Из **корня** репозитория:

~~~bash
docker compose up -d
docker compose ps
docker compose logs -f dns-collector
~~~

Поднимаются три сервиса:

| Сервис | Порт | Назначение |
|---|---|---|
| `postgres` | 5432 | PostgreSQL 16 |
| `dns-collector` | 5353/udp | приём DNS-событий и резолвинг |
| `web-api` | 8080 | REST API и веб-интерфейс |

Веб-интерфейс: `http://localhost:8080`.

> В корневом `docker-compose.yml` порт метрик 9090 не публикуется. Для доступа к `/metrics` используйте production-конфигурацию или добавьте публикацию порта.

## 2. Отправка тестового запроса

~~~bash
echo '{"client_ip":"192.168.1.10","domain":"example.com","qtype":"A","rtype":"dns"}' \
  | nc -u -w1 localhost 5353
~~~

Или Python-клиентом:

~~~bash
python3 test_client.py
~~~

Через секунду проверьте данные:

~~~bash
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT domain, client_ip, rtype, timestamp
FROM domain_stat
ORDER BY timestamp DESC
LIMIT 5;"
~~~

## 3. Проверка резолвинга

Первый батч резолвера запускается сразу при старте, далее — по `interval_seconds` (в dev-конфиге 10 с, в production — 300 с).

~~~bash
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT d.domain, i.ip, i.type, i.time
FROM domain d
JOIN ip i ON i.domain_id = d.id
WHERE d.domain = 'example.com';"
~~~

Если IP ещё нет — подождите батч и проверьте логи:

~~~bash
docker compose logs -f dns-collector | grep -i resolve
~~~

## 4. Нативная сборка

Требования: Go 1.24+, PostgreSQL 16 (локально или в Docker), доступ к DNS.

~~~bash
# 1. Зависимости
cd dns-collector
go mod download

# 2. PostgreSQL (пример через Docker)
docker run -d --name dns-pg \
  -e POSTGRES_USER=dns_collector \
  -e POSTGRES_PASSWORD=dns_collector_pass \
  -e POSTGRES_DB=dns_collector \
  -p 5432:5432 postgres:16-alpine

# 3. Конфиг: host=localhost, пароль из env
export POSTGRES_PASSWORD=dns_collector_pass

# 4. Сборка и запуск
make build
./build/dns-collector -config config/config.yaml
~~~

Миграции применяются автоматически при старте. Если БД недоступна или миграция падает — процесс завершается с ошибкой.

## 5. Конфигурация

Основные параметры `config/config.yaml`:

~~~yaml
server:
  udp_port: 5353

database:
  host: "postgres"      # для нативной сборки — localhost
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

retention:
  stats_days: 30
  cleanup_interval_hours: 24
  ip_ttl_days: 3
  domain_ttl_days: 30   # 0 = не удалять домены по TTL
~~~

После изменения конфига:

~~~bash
docker compose restart dns-collector
# или перезапустить нативный процесс
~~~

## 6. Просмотр данных

Все запросы выполняются через `psql`:

~~~bash
docker compose exec -T postgres psql -U dns_collector -d dns_collector
~~~

Полезные запросы:

~~~sql
-- Сколько доменов и IP
SELECT (SELECT count(*) FROM domain) AS domains,
       (SELECT count(*) FROM ip) AS ips;

-- Топ-10 доменов по числу запросов за сутки
SELECT domain, count(*) AS requests
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours'
GROUP BY domain
ORDER BY requests DESC
LIMIT 10;

-- Домены, ожидающие резолва
SELECT domain, resolv_count, max_resolv, last_resolv_time
FROM domain
WHERE resolv_count < max_resolv
ORDER BY last_resolv_time
LIMIT 20;

-- IP конкретного домена
SELECT d.domain, i.ip, i.type
FROM domain d
JOIN ip i ON i.domain_id = d.id
WHERE d.domain = 'example.com';
~~~

Больше запросов — в [queries.sql](queries.sql).

## 7. Диагностика

~~~bash
# Процесс и порт
ps aux | grep dns-collector
sudo ss -ulnp | grep 5353

# Логи
docker compose logs -f dns-collector
sudo journalctl -u dns-collector -f   # если systemd

# Проверка БД
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "SELECT 1;"
~~~

### Порт занят

~~~bash
sudo lsof -i :5353
# измените server.udp_port в config.yaml
~~~

### DNS не резолвится

- Проверьте `/etc/resolv.conf` внутри контейнера/хоста.
- Увеличьте `resolver.timeout_seconds`.
- Проверьте, что домен вообще есть в `domain` и у него `resolv_count < max_resolv`.
- Помните: IPv4 и IPv6 используют один контекст таймаута; при медленном IPv4 IPv6 может не успеть.

### Нет данных в web-api

- Убедитесь, что `dns-collector` пишет в ту же БД (`database` в обоих конфигах).
- Проверьте `domain_stat` напрямую.
- Проверьте `docker compose logs web-api`.

## 8. Остановка и очистка

~~~bash
docker compose down          # остановить, данные сохранить
docker compose down -v       # остановить и удалить volume PostgreSQL
~~~

## 9. Что дальше

- [ARCHITECTURE.md](ARCHITECTURE.md) — как устроены компоненты и схема.
- [INSTALL.md](INSTALL.md) — нативная установка, systemd, обновление.
- [DOCKER.md](DOCKER.md) — Docker, бэкапы, метрики.
- [../docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md) — план снижения нагрузки на диск.
