# DNS Collector Web API

Web-микросервис для визуализации и анализа данных DNS Collector.

## Возможности

### Статистика DNS-запросов
- Просмотр всех DNS-запросов с пагинацией
- Фильтрация по IP клиента (один или несколько)
- Фильтрация по подсети (CIDR нотация)
- Фильтрация по диапазону дат
- Сортировка по любому полю (ID, домен, IP клиента, тип, время)

### Просмотр доменов
- Список всех доменов с информацией о резолвинге
- Фильтрация по имени домена с поддержкой регулярных выражений
- Фильтрация по диапазону дат
- Сортировка по любому полю
- Просмотр всех зарезолвленных IP адресов для каждого домена
- Отображение IPv4 и IPv6 адресов

### Excel экспорт (v2.3.2+)
- Экспорт статистики DNS-запросов в Excel (XLSX)
- Экспорт доменов с IP адресами (двухлистовой файл)
- Полное форматирование: жирные заголовки, закрепление панелей, автофильтры
- Применение всех фильтров к экспортируемым данным
- Лимит 100,000 записей с HTTP 413 при превышении
- Кнопки экспорта интегрированы в веб-интерфейс

### Экспорт списков для pfSense (v2.3.0+)
- Экспорт доменов и IP адресов в plain text формате
- Фильтрация по regex (PostgreSQL синтаксис)
- Настраиваемое включение/выключение доменов
- Поддержка множественных списков
- Защита от ReDoS атак
- HTTP кеширование (5 минут)
- Подробнее: [EXPORT_LISTS.md](EXPORT_LISTS.md)

## Архитектура

```
web-api/
├── cmd/                    # Точка входа приложения
│   └── main.go
├── internal/               # Внутренние пакеты
│   ├── database/          # Слой работы с БД
│   ├── handlers/          # HTTP handlers
│   └── models/            # Модели данных
├── frontend/              # Vue.js фронтенд
│   ├── src/
│   │   ├── api/          # API клиент
│   │   ├── components/   # Vue компоненты
│   │   ├── views/        # Страницы
│   │   └── App.vue
│   └── package.json
├── config/                # Конфигурация
│   └── config.yaml
├── Dockerfile
└── go.mod
```

## API Endpoints

### GET /api/stats
Получение статистики DNS-запросов

**Query параметры:**
- `client_ips` - список IP адресов через запятую (опционально)
- `subnet` - подсеть в CIDR формате (опционально)
- `date_from` - начало диапазона дат в ISO8601 (опционально)
- `date_to` - конец диапазона дат в ISO8601 (опционально)
- `sort_by` - поле для сортировки: id, domain, client_ip, rtype, timestamp (по умолчанию: timestamp)
- `sort_order` - порядок сортировки: asc, desc (по умолчанию: desc)
- `limit` - количество записей (по умолчанию: 100)
- `offset` - смещение для пагинации (по умолчанию: 0)

**Примеры:**
```bash
# Все запросы
curl "http://localhost:8080/api/stats"

# Запросы от конкретного IP
curl "http://localhost:8080/api/stats?client_ips=192.168.1.10"

# Запросы от нескольких IP
curl "http://localhost:8080/api/stats?client_ips=192.168.1.10,192.168.1.20"

# Запросы из подсети
curl "http://localhost:8080/api/stats?subnet=192.168.1.0/24"

# Запросы за последний час
curl "http://localhost:8080/api/stats?date_from=2024-12-17T10:00:00Z&date_to=2024-12-17T11:00:00Z"

# С сортировкой и пагинацией
curl "http://localhost:8080/api/stats?sort_by=domain&sort_order=asc&limit=50&offset=0"
```

### GET /api/domains
Получение списка доменов

**Query параметры:**
- `domain_regex` - регулярное выражение для фильтрации доменов (опционально)
- `date_from` - начало диапазона дат в ISO8601 (опционально)
- `date_to` - конец диапазона дат в ISO8601 (опционально)
- `sort_by` - поле для сортировки: id, domain, time_insert, resolv_count, max_resolv, last_resolv_time
- `sort_order` - порядок сортировки: asc, desc (по умолчанию: desc)
- `limit` - количество записей (по умолчанию: 100)
- `offset` - смещение для пагинации

**Примеры:**
```bash
# Все домены
curl "http://localhost:8080/api/domains"

# Домены, содержащие "google"
curl "http://localhost:8080/api/domains?domain_regex=.*google.*"

# Домены, начинающиеся с "mail"
curl "http://localhost:8080/api/domains?domain_regex=^mail\\."

# Все .com домены
curl "http://localhost:8080/api/domains?domain_regex=\\.com$"

# С сортировкой по количеству резолвингов
curl "http://localhost:8080/api/domains?sort_by=resolv_count&sort_order=desc"
```

### GET /api/domains/:id
Получение информации о домене со всеми IP адресами

**Пример:**
```bash
curl "http://localhost:8080/api/domains/1"
```

### GET /api/stats/export
Экспорт статистики DNS-запросов в Excel (v2.3.2+)

**Query параметры:** те же, что и для `/api/stats` (client_ips, subnet, date_from, date_to, sort_by, sort_order)

**Response:** Excel файл (.xlsx) с одним листом "DNS Statistics"

**Колонки:**
- ID - уникальный идентификатор записи
- Domain - доменное имя
- Client IP - IP адрес клиента
- Record Type - тип записи DNS
- Timestamp - время запроса (формат: yyyy-mm-dd hh:mm:ss)

**Особенности:**
- Жирные заголовки с синим фоном
- Закрепление первой строки (freeze panes)
- Автофильтры на всех колонках
- Оптимизированная ширина колонок
- Лимит: 100,000 записей (при превышении возвращается HTTP 413)

**Примеры:**
```bash
# Экспорт всей статистики
curl -o stats.xlsx "http://localhost:8080/api/stats/export"

# Экспорт с фильтрами
curl -o stats_filtered.xlsx "http://localhost:8080/api/stats/export?client_ips=192.168.1.10&date_from=2024-12-01T00:00:00Z"

# Экспорт из подсети
curl -o stats_subnet.xlsx "http://localhost:8080/api/stats/export?subnet=192.168.1.0/24"
```

### GET /api/domains/export
Экспорт доменов с IP адресами в Excel (v2.3.2+)

**Query параметры:** те же, что и для `/api/domains` (domain_regex, date_from, date_to, sort_by, sort_order)

**Response:** Excel файл (.xlsx) с двумя листами

**Лист 1 "Domains":**
- ID - уникальный идентификатор домена
- Domain - доменное имя
- First Seen - время первого появления
- Resolution Count - количество выполненных резолвингов
- Max Resolutions - максимальное количество резолвингов
- Last Resolved - время последнего резолвинга

**Лист 2 "IP Addresses":**
- Domain - доменное имя
- IP Address - IP адрес
- Type - тип адреса (IPv4 или IPv6)
- Resolved At - время резолвинга

**Особенности:**
- Полное форматирование на обоих листах
- Оптимизация производительности: bulk fetch IP адресов (один SQL запрос)
- Лимит: 100,000 записей (при превышении возвращается HTTP 413)

**Примеры:**
```bash
# Экспорт всех доменов
curl -o domains.xlsx "http://localhost:8080/api/domains/export"

# Экспорт доменов, содержащих "google"
curl -o google_domains.xlsx "http://localhost:8080/api/domains/export?domain_regex=.*google.*"

# Экспорт .com доменов
curl -o com_domains.xlsx "http://localhost:8080/api/domains/export?domain_regex=\\.com$"
```

### GET /health
Health check endpoint

### GET /export/{endpoint}
Экспорт списков доменов и IP адресов для pfSense (v2.3.0+)

**Конфигурация** (`config/config.yaml`):
```yaml
export_lists:
  - name: "Blocked Domains"
    endpoint: "/export/blocked"
    domain_regex: "^(ads|tracking)\\."
    include_domains: true
```

**Пример:**
```bash
# Получение списка
curl "http://localhost:8080/export/blocked"

# Вывод (plain text):
# ads.example.com
# tracking.analytics.com
# 192.0.2.1
# 192.0.2.2
# 2001:db8::1
```

**Параметры конфигурации:**
- `name` - Название списка (обязательно)
- `endpoint` - HTTP endpoint (обязательно, должен начинаться с `/`)
- `domain_regex` - PostgreSQL regex для фильтрации (обязательно, ≤200 символов)
- `include_domains` - Включать домены в вывод (обязательно, true/false)

**Безопасность:**
- Защита от ReDoS: блокируются паттерны `(.*)*`, `(.+)+`, `(.*)+`, `(.+)*`
- Валидация конфигурации при старте
- HTTP кеширование: `Cache-Control: public, max-age=300`

Подробная документация: [EXPORT_LISTS.md](EXPORT_LISTS.md)

## Запуск в разработке

### Backend
```bash
cd web-api
go mod download
go run cmd/main.go -config config/config.yaml
```

### Frontend
```bash
cd web-api/frontend
npm install
npm run dev
```

Фронтенд будет доступен на http://localhost:5173

## Запуск с Docker

```bash
cd web-api
docker build -t dns-collector-webapi .
docker run -p 8080:8080 -v $(pwd)/../data:/app/data dns-collector-webapi
```

## Конфигурация

Файл `config/config.yaml`:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  domains_db: "../data/domains.db"
  stats_db: "../data/stats.db"

logging:
  level: "info"

cors:
  allowed_origins:
    - "http://localhost:8080"
    - "http://localhost:5173"
  allow_credentials: true
```

## Технологии

**Backend:**
- Go 1.21
- Gin Web Framework
- SQLite3

**Frontend:**
- Vue.js 3
- Vue Router
- Axios
- Vite

## Примеры использования регулярных выражений

В фильтре доменов можно использовать любые регулярные выражения Go:

- `^google\.` - домены, начинающиеся с "google."
- `\.com$` - все .com домены
- `^mail\.|^smtp\.` - домены, начинающиеся с "mail." или "smtp."
- `.*cloudflare.*` - домены, содержащие "cloudflare"
- `^[0-9]` - домены, начинающиеся с цифры

## База данных

Сервис работает с двумя SQLite базами в режиме read-only:
- `domains.db` - информация о доменах и их IP адресах
- `stats.db` - статистика DNS запросов

Базы данных создаются и наполняются основным сервисом dns-collector.
