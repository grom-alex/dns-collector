# DNS Collector - Архитектура микросервисов

## Обзор

DNS Collector состоит из двух независимых микросервисов:

1. **dns-collector** - основной сервис сбора и резолвинга DNS запросов
2. **web-api** - веб-интерфейс для визуализации и анализа данных

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                     Docker Compose                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────┐         ┌────────────────────┐   │
│  │   dns-collector      │         │     web-api        │   │
│  │   (UDP Server +      │         │  (HTTP Server +    │   │
│  │    DNS Resolver)     │         │   Vue.js UI)       │   │
│  │                      │         │                    │   │
│  │  Port: 5353/udp     │         │  Port: 8080/tcp    │   │
│  └──────────┬───────────┘         └──────────┬─────────┘   │
│             │                                 │             │
│             │    ┌─────────────┐             │             │
│             └───▶│    data/    │◀────────────┘             │
│                  │             │                           │
│                  │ domains.db  │  (shared volume, RO)     │
│                  │  stats.db   │                           │
│                  └─────────────┘                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Сервис: dns-collector

### Назначение
Сбор DNS запросов от клиентов и периодический резолвинг доменов.

### Компоненты
- **UDP Server** (port 5353): принимает JSON сообщения с DNS запросами
- **DNS Resolver**: периодически резолвит домены в IP адреса
- **SQLite БД**: хранит домены, IP адреса и статистику

### Технологии
- Go 1.21
- SQLite3
- UDP протокол

### Конфигурация
Файл: `config.yaml`

```yaml
server:
  udp_port: 5353

database:
  domains_db: "data/domains.db"
  stats_db: "data/stats.db"

resolver:
  interval_seconds: 300
  max_resolv: 10
  timeout_seconds: 5
  workers: 5
```

### Запуск
```bash
# Локально
go run cmd/dns-collector/main.go -config config.yaml

# Docker
docker run -p 5353:5353/udp \
  -v $(pwd)/data:/app/data \
  dns-collector:latest
```

### Документация
- [README.md](README.md) - основная документация
- [ARCHITECTURE.md](ARCHITECTURE.md) - архитектура сервиса
- [INSTALL.md](INSTALL.md) - установка и настройка

## Сервис: web-api

### Назначение
Веб-интерфейс для просмотра и анализа собранных DNS данных.

### Компоненты
- **REST API** (Go + Gin): обработка запросов к данным
- **Web UI** (Vue.js): интерактивный интерфейс пользователя
- **SQLite Reader**: read-only доступ к БД

### Технологии
**Backend:**
- Go 1.21
- Gin Web Framework
- SQLite3 (read-only)

**Frontend:**
- Vue.js 3
- Vue Router
- Axios
- Vite

### Возможности

#### 1. Статистика DNS запросов
- Просмотр всех DNS запросов
- Фильтрация по IP клиента (один или несколько)
- Фильтрация по подсети (CIDR)
- Фильтрация по диапазону дат
- Сортировка по любому полю
- Пагинация

#### 2. Просмотр доменов
- Список всех доменов
- Фильтрация по регулярным выражениям
- Фильтрация по диапазону дат
- Просмотр зарезолвленных IP для каждого домена
- Разделение IPv4/IPv6
- Сортировка и пагинация

### API Endpoints

```
GET  /health              - Health check
GET  /api/stats           - Статистика DNS запросов
GET  /api/domains         - Список доменов
GET  /api/domains/:id     - Детали домена с IP адресами
```

### Конфигурация
Файл: `web-api/config/config.yaml`

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  domains_db: "/app/data/domains.db"
  stats_db: "/app/data/stats.db"

cors:
  allowed_origins:
    - "http://localhost:8080"
```

### Запуск
```bash
# Локально (backend)
cd web-api
go run cmd/main.go -config config/config.yaml

# Локально (frontend dev-server)
cd web-api/frontend
npm install
npm run dev

# Docker
docker build -t dns-collector-webapi ./web-api
docker run -p 8080:8080 \
  -v $(pwd)/data:/app/data:ro \
  dns-collector-webapi:latest
```

### Документация
- [web-api/README.md](web-api/README.md) - описание API и возможностей
- [web-api/INSTALL.md](web-api/INSTALL.md) - установка и отладка

## Общие ресурсы

### База данных

Оба сервиса используют общие SQLite базы:

**domains.db:**
- `domain` - таблица доменов
- `ip` - таблица IP адресов

**stats.db:**
- `domain_stat` - статистика DNS запросов

### Разделение доступа
- **dns-collector**: read-write доступ, создает и обновляет данные
- **web-api**: read-only доступ, только читает данные

### Volume mapping
```yaml
volumes:
  - ./data:/app/data        # dns-collector (RW)
  - ./data:/app/data:ro     # web-api (RO)
```

## Запуск всей системы

### Docker Compose (рекомендуется)

```bash
# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f

# Остановка
docker-compose down
```

### Порты
- `5353/udp` - dns-collector UDP server
- `8080/tcp` - web-api HTTP server

### Проверка работы

```bash
# Health check dns-collector (через БД)
sqlite3 data/domains.db "SELECT COUNT(*) FROM domain;"

# Health check web-api
curl http://localhost:8080/health

# Отправка тестового запроса в dns-collector
echo '{"client_ip":"192.168.1.10","domain":"google.com","qtype":"A","rtype":"dns"}' | \
  nc -u -w1 localhost 5353

# Просмотр статистики через web-api
curl http://localhost:8080/api/stats?limit=5
```

## Мониторинг

### Логи

```bash
# dns-collector
docker logs -f dns-collector

# web-api
docker logs -f dns-collector-webapi
```

### Метрики

```bash
# Количество доменов
sqlite3 data/domains.db "SELECT COUNT(*) FROM domain;"

# Количество запросов
sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"

# Последние запросы
sqlite3 data/stats.db \
  "SELECT domain, client_ip, datetime(timestamp)
   FROM domain_stat
   ORDER BY timestamp DESC
   LIMIT 10;"
```

### Health Checks

Docker Compose автоматически проверяет здоровье сервисов:

```yaml
# dns-collector
healthcheck:
  test: ["CMD", "sqlite3", "/app/data/domains.db", "SELECT 1;"]
  interval: 30s

# web-api
healthcheck:
  test: ["CMD", "wget", "--quiet", "--spider", "http://localhost:8080/health"]
  interval: 30s
```

## Масштабирование

### Вертикальное

Увеличение ресурсов в `docker-compose.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '2.0'
      memory: 1G
```

### Горизонтальное

**dns-collector:**
- Требует переход на централизованную БД (PostgreSQL)
- Балансировщик для UDP трафика
- Координация через distributed locks

**web-api:**
- Легко масштабируется (read-only)
- Добавить nginx/haproxy для балансировки
- Можно запустить несколько инстансов

```bash
docker-compose up -d --scale web-api=3
```

## Безопасность

### dns-collector
- Валидация входящих данных
- Ограничение размера UDP пакетов
- Firewall для порта 5353

### web-api
- Read-only доступ к БД
- CORS конфигурация
- Rate limiting (через nginx/traefik)

### Сеть
```yaml
# В docker-compose.yml можно добавить сеть
networks:
  dns-collector-net:
    driver: bridge
```

## Backup

### Автоматический backup БД

```bash
#!/bin/bash
# backup.sh
DATE=$(date +%Y%m%d_%H%M%S)
cp data/domains.db backups/domains_${DATE}.db
cp data/stats.db backups/stats_${DATE}.db
```

### Очистка старых данных

```bash
# Удаление статистики старше 30 дней
sqlite3 data/stats.db \
  "DELETE FROM domain_stat WHERE timestamp < datetime('now', '-30 days');"

# Вакуумирование
sqlite3 data/stats.db "VACUUM;"
```

## Обновление

### dns-collector

```bash
docker pull registry.gromas.ru/apps/dns-collector:latest
docker-compose up -d dns-collector
```

### web-api

```bash
cd web-api
docker build -t dns-collector-webapi:latest .
docker-compose up -d web-api
```

## Troubleshooting

### dns-collector не получает запросы
```bash
# Проверка порта
netstat -uln | grep 5353

# Тест отправки
echo '{"domain":"test.com"}' | nc -u -w1 localhost 5353

# Логи
docker logs dns-collector
```

### web-api показывает пустые данные
```bash
# Проверка БД
ls -la data/
sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"

# Проверка прав доступа
docker exec dns-collector-webapi ls -la /app/data/

# Логи
docker logs dns-collector-webapi
```

### Проблемы производительности
```bash
# Размер БД
du -h data/*.db

# Количество записей
sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"

# Очистка старых данных
# См. раздел Backup выше
```

## Дополнительные материалы

- [README.md](README.md) - общее описание проекта
- [ARCHITECTURE.md](ARCHITECTURE.md) - подробная архитектура dns-collector
- [web-api/README.md](web-api/README.md) - документация web-api
- [queries.sql](queries.sql) - полезные SQL запросы
