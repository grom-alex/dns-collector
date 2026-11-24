# Установка и запуск dns-collector

## Требования

- Go 1.24+ (модуль объявляет `go 1.24.0`, toolchain 1.24.4).
- PostgreSQL 16 (локально, в Docker или managed).
- Доступ к DNS-резолверу (системный `/etc/resolv.conf`).
- Свободный UDP-порт (по умолчанию 5353).

SQLite больше не используется. Все данные хранятся в PostgreSQL.

## Вариант 1. Docker Compose

См. [QUICKSTART.md](QUICKSTART.md) и [DOCKER.md](DOCKER.md). Кратко:

~~~bash
docker compose up -d
~~~

## Вариант 2. Нативная сборка

### 1. Установить Go

~~~bash
# macOS
brew install go

# Ubuntu/Debian — официальный tarball
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
~~~

### 2. Подготовить PostgreSQL

~~~sql
CREATE USER dns_collector WITH PASSWORD 'strong-password';
CREATE DATABASE dns_collector OWNER dns_collector;
~~~

Для managed PostgreSQL убедитесь, что у роли есть права на создание таблиц и индексов. Миграция `000003` делает `ALTER DATABASE` и требует владельца БД; при недостатке прав сервис не стартует (см. план оптимизации).

### 3. Собрать

~~~bash
cd dns-collector
go mod download
make build
# бинарник: build/dns-collector
~~~

Или без Makefile:

~~~bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o build/dns-collector ./cmd/dns-collector
~~~

### 4. Настроить конфиг

Скопируйте `config/config.yaml` и измените:

- `database.host` — адрес PostgreSQL;
- `database.password` — пусто, если используете `POSTGRES_PASSWORD`;
- `server.udp_port` — при необходимости;
- `resolver`/`retention` — под нагрузку и политику хранения.

~~~bash
export POSTGRES_PASSWORD='strong-password'
export POSTGRES_SSL_MODE='require'   # если БД с SSL
~~~

### 5. Запустить

~~~bash
./build/dns-collector -config config/config.yaml
~~~

При старте:

1. Читается конфиг и валидируются значения.
2. Открывается пул подключений (`max_open_conns = 25`).
3. Создаётся схема (`initSchema`) и применяются миграции.
4. Запускаются метрики, UDP-сервер, резолвер и cleanup.
5. Процесс ждёт `SIGINT`/`SIGTERM` и завершается graceful.

## systemd

`/etc/systemd/system/dns-collector.service`:

~~~ini
[Unit]
Description=DNS Collector
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=dns-collector
Group=dns-collector
WorkingDirectory=/opt/dns-collector
Environment=POSTGRES_PASSWORD=strong-password
Environment=POSTGRES_SSL_MODE=disable
ExecStart=/opt/dns-collector/build/dns-collector -config /opt/dns-collector/config/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
~~~

~~~bash
sudo systemctl daemon-reload
sudo systemctl enable --now dns-collector
sudo systemctl status dns-collector
sudo journalctl -u dns-collector -f
~~~

## Проверка

~~~bash
# UDP-порт
sudo ss -ulnp | grep 5353

# Тестовый пакет
echo '{"client_ip":"192.168.1.10","domain":"example.com","qtype":"A","rtype":"dns"}' \
  | nc -u -w1 127.0.0.1 5353

# Данные в БД
psql "$DATABASE_URL" -c "SELECT count(*) FROM domain_stat;"
psql "$DATABASE_URL" -c "SELECT domain, resolv_count FROM domain ORDER BY id DESC LIMIT 5;"
~~~

## Обновление

1. Сделайте бэкап: `pg_dump -Fc -f backup.dump dns_collector`.
2. Соберите новый бинарник/образ.
3. Остановите сервис, замените бинарник, запустите.
4. Миграции применятся автоматически; при ошибке — откатитесь на предыдущий бинарник и восстановите дамп.

## Бэкап и восстановление

~~~bash
# Бэкап
pg_dump -Fc -f dns_collector_$(date +%F).dump dns_collector

# Восстановление в пустую БД
createdb -O dns_collector dns_collector_restore
pg_restore -d dns_collector_restore dns_collector_$(date +%F).dump
~~~

Для Docker — см. [DOCKER.md](DOCKER.md).

## Производительность

- Резолвер использует пул воркеров (`resolver.workers`).
- `interval_seconds` задаёт частоту батчей, но не ограничивает длительность: батчи могут накладываться.
- Для больших объёмов:
  - PostgreSQL на SSD/NVMe, `shared_buffers` ≥ 25% RAM;
  - не менее 2–4 ГБ RAM для БД;
  - отдельный диск/пул под WAL и данные (опционально);
  - мониторинг записи и HOT-обновлений (см. [../docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md)).

## Troubleshooting

### `failed to ping database`

- Проверьте host/port/user/password и `pg_hba.conf`.
- В Docker host — `postgres`, не `localhost`.
- SSL: `sslmode=disable` для локальной БД, `require` — для SSL.

### `database is in dirty state`

Миграция упала на середине. Посмотрите `schema_migrations`, устраните причину, выполните `migrate force <version>` вручную (с бэкапом) и перезапустите.

### `permission denied for database` в миграции 000003

Роль не владелец БД и не может выполнить `ALTER DATABASE`. Дайте владение или удалите/перепишите миграцию (план: убрать `ALTER DATABASE` из миграций).

### Домен не резолвится

- Проверьте `/etc/resolv.conf`.
- Увеличьте `timeout_seconds`.
- Проверьте, что домен есть в `domain`.
- Помните про общий контекст IPv4/IPv6.
