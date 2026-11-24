# Установка и запуск DNS Collector Web API

## Быстрый старт с Docker Compose

Из корневой директории проекта dns-collector:

```bash
# Сборка и запуск всех сервисов
docker compose up -d

# Просмотр логов
docker compose logs -f web-api

# Остановка сервисов
docker compose down
```

Web-интерфейс будет доступен по адресу: http://localhost:8080

## Разработка

### Предварительные требования

- Go 1.24+
- Node.js 20+
- npm или yarn

### Backend

```bash
cd web-api

# Установка зависимостей
go mod download

# Запуск в режиме разработки
go run cmd/main.go -config config/config.yaml
```

Для локальной разработки создайте файл `config/config.dev.yaml`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  host: "localhost"
  port: 5432
  user: "dns_collector"
  password: ""          # POSTGRES_PASSWORD
  database: "dns_collector"
  ssl_mode: "disable"

logging:
  level: "debug"

cors:
  allowed_origins:
    - "http://localhost:5173"
    - "http://localhost:8080"
  allow_credentials: true
```

Запуск с dev-конфигом:

```bash
go run cmd/main.go -config config/config.dev.yaml
```

### Frontend

```bash
cd web-api/frontend

# Установка зависимостей
npm install

# Запуск dev-сервера с hot-reload
npm run dev

# Сборка для продакшена
npm run build
```

Dev-сервер запустится на http://localhost:5173 с проксированием API запросов на backend.

## Сборка Docker образа

```bash
cd web-api

# Сборка образа
docker build -t dns-collector-webapi:latest .

# Запуск контейнера (нужен доступ к PostgreSQL)
docker run -d \
  -p 8080:8080 \
  -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  -e POSTGRES_SSL_MODE=disable \
  --name dns-collector-webapi \
  dns-collector-webapi:latest

# В config/config.yaml database.host должен указывать на доступный PostgreSQL
# (в compose-сети — "postgres", при запуске на хосте — "localhost").
```

## Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/health

# Получение статистики
curl http://localhost:8080/api/stats?limit=10

# Получение доменов
curl http://localhost:8080/api/domains?limit=10
```

## Структура проекта

```
web-api/
├── cmd/
│   └── main.go                 # Точка входа приложения
├── internal/
│   ├── database/               # Работа с БД
│   │   └── database.go
│   ├── handlers/               # HTTP handlers
│   │   └── handlers.go
│   └── models/                 # Модели данных
│       └── models.go
├── frontend/                   # Vue.js приложение
│   ├── src/
│   │   ├── api/               # API клиент
│   │   ├── views/             # Страницы
│   │   ├── App.vue            # Главный компонент
│   │   └── main.js            # Точка входа
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── config/
│   └── config.yaml            # Конфигурация
├── Dockerfile                 # Multi-stage build
├── go.mod
└── README.md
```

## Отладка

### Backend логи

```bash
# Docker
docker logs -f dns-collector-webapi

# Локально - уровень debug
# В config.yaml установите:
logging:
  level: "debug"
```

### Проверка доступа к БД

```bash
# PostgreSQL доступен
pg_isready -h localhost -p 5432 -U dns_collector

# Проверка содержимого
psql "postgres://dns_collector:$POSTGRES_PASSWORD@localhost:5432/dns_collector" \
  -c "SELECT COUNT(*) FROM domain;"
psql "postgres://dns_collector:$POSTGRES_PASSWORD@localhost:5432/dns_collector" \
  -c "SELECT COUNT(*) FROM domain_stat;"
```

### Frontend отладка

В браузере откройте DevTools (F12) для просмотра:
- Network - API запросы
- Console - ошибки JavaScript
- Vue DevTools - состояние компонентов (если установлено расширение)

## Производительность

### Оптимизация запросов

API поддерживает пагинацию. Рекомендуемые значения:
- `limit`: 50-200 записей
- Используйте `offset` для навигации по страницам

### Индексы БД

Схему и индексы создают миграции (общие с dns-collector):
- `domain_stat.timestamp` — фильтрация по дате;
- `domain_stat.domain`, `domain_stat.client_ip` — фильтры UI;
- `ip.domain_id` — выборка IP домена;
- `domain.domain` — уникальность и поиск.

Список индексов и план их сокращения: [../docs/OPTIMIZATION_PLAN.md](../docs/OPTIMIZATION_PLAN.md#22-индексы-которые-убивают-hot).

## Безопасность

Web API читает данные, но **не является read-only на уровне БД**: при старте он применяет миграции (DDL). Кроме того, **встроенной аутентификации нет** — сервис нельзя выставлять в интернет без reverse proxy с auth. CORS не заменяет аутентификацию.

### Настройка CORS

В `config.yaml`:

```yaml
cors:
  allowed_origins:
    - "https://your-domain.com"
  allow_credentials: true
```

## Troubleshooting

### Ошибка подключения к PostgreSQL

1. Проверьте `database.host`: в Docker это `postgres`, локально — `localhost`.
2. Проверьте `POSTGRES_PASSWORD`, `POSTGRES_SSL_MODE` и `pg_hba.conf`.
3. Убедитесь, что PostgreSQL healthy: `docker compose ps`, `pg_isready`.
4. Проверьте, что миграции не упали: `docker compose logs web-api`.

### Фронтенд не загружается

1. Проверьте что frontend был собран:
   ```bash
   ls -la frontend/dist/
   ```

2. Пересоберите frontend:
   ```bash
   cd frontend
   npm run build
   ```

3. В Docker пересоберите образ:
   ```bash
   docker compose build web-api
   ```

### API возвращает пустые данные

1. Убедитесь что dns-collector работает и собирает данные
2. Проверьте `database` в конфигурации (host/port/user/database).
3. Проверьте содержимое БД:
   ```bash
   docker compose exec -T postgres psql -U dns_collector -d dns_collector \
     -c "SELECT COUNT(*) FROM domain_stat;"
   ```
