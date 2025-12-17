# Быстрый старт Web API

## Запуск с Docker Compose (рекомендуется)

Самый простой способ - запустить все сервисы одной командой:

```bash
# Из корневой директории проекта
docker-compose up -d
```

Это запустит:
- `dns-collector` на порту `5353/udp`
- `web-api` на порту `8080/tcp`

Откройте браузер: **http://localhost:8080**

## Проверка работы

### 1. Отправка тестовых DNS запросов

```bash
# Отправка одного запроса
echo '{"client_ip":"192.168.1.10","domain":"google.com","qtype":"A","rtype":"dns"}' | nc -u -w1 localhost 5353

# Отправка нескольких запросов
for domain in google.com yandex.ru mail.ru github.com; do
  echo "{\"client_ip\":\"192.168.1.10\",\"domain\":\"$domain\",\"qtype\":\"A\",\"rtype\":\"dns\"}" | nc -u -w1 localhost 5353
  sleep 1
done
```

### 2. Проверка через Web UI

Откройте http://localhost:8080 и перейдите на вкладки:
- **Statistics** - просмотр DNS запросов
- **Domains** - просмотр доменов и их IP адресов

### 3. Проверка через API

```bash
# Health check
curl http://localhost:8080/health

# Последние 10 DNS запросов
curl "http://localhost:8080/api/stats?limit=10" | jq

# Все домены
curl "http://localhost:8080/api/domains?limit=10" | jq

# Поиск доменов по regex (все .com домены)
curl "http://localhost:8080/api/domains?domain_regex=\\.com$" | jq
```

## Использование фильтров

### Фильтрация статистики

```bash
# Запросы от конкретного IP
curl "http://localhost:8080/api/stats?client_ips=192.168.1.10"

# Запросы из подсети
curl "http://localhost:8080/api/stats?subnet=192.168.1.0/24"

# Запросы за последний час
START=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)
END=$(date -u +%Y-%m-%dT%H:%M:%SZ)
curl "http://localhost:8080/api/stats?date_from=$START&date_to=$END"

# С сортировкой
curl "http://localhost:8080/api/stats?sort_by=domain&sort_order=asc&limit=20"
```

### Фильтрация доменов

```bash
# Домены, содержащие "google"
curl "http://localhost:8080/api/domains?domain_regex=.*google.*"

# Домены, начинающиеся с "mail"
curl "http://localhost:8080/api/domains?domain_regex=^mail\\."

# Все .ru домены
curl "http://localhost:8080/api/domains?domain_regex=\\.ru$"
```

## Мониторинг

### Просмотр логов

```bash
# Все логи
docker-compose logs -f

# Только web-api
docker-compose logs -f web-api

# Только dns-collector
docker-compose logs -f dns-collector
```

### Статус сервисов

```bash
docker-compose ps
```

### Проверка БД

```bash
# Количество доменов
sqlite3 data/domains.db "SELECT COUNT(*) FROM domain;"

# Количество запросов
sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"

# Последние 5 запросов
sqlite3 data/stats.db "SELECT domain, client_ip, datetime(timestamp) FROM domain_stat ORDER BY timestamp DESC LIMIT 5;"
```

## Остановка

```bash
# Остановить все сервисы
docker-compose down

# Остановить и удалить volumes
docker-compose down -v
```

## Разработка

### Запуск backend локально

```bash
cd web-api
go mod download
go run cmd/main.go -config config/config.yaml
```

### Запуск frontend dev-server

```bash
cd web-api/frontend
npm install
npm run dev
```

Frontend dev-server: http://localhost:5173

## Примеры использования

### Пример 1: Мониторинг DNS активности домена

```bash
# В Web UI:
# 1. Откройте Statistics
# 2. В поле "Client IPs" введите: 192.168.1.10
# 3. Нажмите "Apply Filters"
# 4. Отсортируйте по времени (click на "Timestamp")
```

### Пример 2: Поиск всех поддоменов Google

```bash
# Через API
curl "http://localhost:8080/api/domains?domain_regex=.*google.*" | jq

# Или в Web UI:
# 1. Откройте Domains
# 2. В поле "Domain Regex Pattern" введите: .*google.*
# 3. Нажмите "Apply Filters"
```

### Пример 3: Просмотр IP адресов домена

```bash
# В Web UI:
# 1. Откройте Domains
# 2. Найдите нужный домен
# 3. Нажмите "Show IPs"
# 4. Увидите все IPv4 и IPv6 адреса с датами резолвинга
```

## Troubleshooting

### Web UI не загружается

```bash
# Проверьте статус контейнера
docker ps | grep webapi

# Проверьте логи
docker logs dns-collector-webapi

# Пересоберите образ
docker-compose build web-api
docker-compose up -d web-api
```

### Пустые данные

```bash
# Убедитесь что dns-collector работает
docker logs dns-collector

# Проверьте БД
ls -la data/
sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"

# Отправьте тестовые данные
echo '{"client_ip":"192.168.1.10","domain":"test.com","qtype":"A","rtype":"dns"}' | nc -u -w1 localhost 5353
```

### Ошибка подключения к БД

```bash
# Проверьте что volume смонтирован
docker inspect dns-collector-webapi | grep Mounts -A 10

# Проверьте права доступа
ls -la data/

# Пересоздайте контейнер
docker-compose up -d --force-recreate web-api
```

## Дополнительная информация

- [README.md](README.md) - основная документация проекта
- [SERVICES.md](SERVICES.md) - архитектура микросервисов
- [web-api/README.md](web-api/README.md) - документация API
- [web-api/INSTALL.md](web-api/INSTALL.md) - подробная установка

## Полезные команды

```bash
# Перезапуск сервисов
docker-compose restart

# Обновление образов
docker-compose pull
docker-compose up -d

# Просмотр использования ресурсов
docker stats dns-collector dns-collector-webapi

# Backup БД
cp data/domains.db data/domains_backup_$(date +%Y%m%d).db
cp data/stats.db data/stats_backup_$(date +%Y%m%d).db

# Очистка старых данных (> 30 дней)
sqlite3 data/stats.db "DELETE FROM domain_stat WHERE timestamp < datetime('now', '-30 days');"
sqlite3 data/stats.db "VACUUM;"
```
