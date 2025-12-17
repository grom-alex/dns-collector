# Установка и запуск DNS Collector Web API

## Быстрый старт с Docker Compose

Из корневой директории проекта dns-collector:

```bash
# Сборка и запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f web-api

# Остановка сервисов
docker-compose down
```

Web-интерфейс будет доступен по адресу: http://localhost:8080

## Разработка

### Предварительные требования

- Go 1.21+
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
  domains_db: "../data/domains.db"
  stats_db: "../data/stats.db"

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

# Запуск контейнера
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/../data:/app/data:ro \
  --name dns-collector-webapi \
  dns-collector-webapi:latest
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
# Проверка что БД доступны
ls -la ../data/

# Проверка содержимого
sqlite3 ../data/domains.db "SELECT COUNT(*) FROM domain;"
sqlite3 ../data/stats.db "SELECT COUNT(*) FROM domain_stat;"
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

Основной сервис dns-collector создает необходимые индексы:
- `domain_stat.timestamp` - для быстрой фильтрации по дате
- `domain_stat.domain` - для поиска по домену
- `ip.domain_id` - для быстрого получения IP домена

## Безопасность

Web API работает в read-only режиме с базами данных:
- Открывает БД с флагом `?mode=ro`
- Не может модифицировать данные
- Безопасно для публичного доступа (с учетом CORS)

### Настройка CORS

В `config.yaml`:

```yaml
cors:
  allowed_origins:
    - "https://your-domain.com"
  allow_credentials: true
```

## Troubleshooting

### Ошибка "database is locked"

Web API использует read-only режим, конфликтов быть не должно. Если ошибка возникает:

1. Проверьте что основной сервис dns-collector использует WAL mode
2. Убедитесь что volume монтирован корректно
3. Проверьте права доступа к файлам БД

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
   docker-compose build web-api
   ```

### API возвращает пустые данные

1. Убедитесь что dns-collector работает и собирает данные
2. Проверьте путь к БД в конфигурации
3. Проверьте содержимое БД:
   ```bash
   sqlite3 data/stats.db "SELECT COUNT(*) FROM domain_stat;"
   ```
