# DNS Collector

Микросервисная система для сбора DNS запросов, резолвинга доменов и визуализации данных.

![Build Status](https://github.com/grom-alex/dns-collector/actions/workflows/build.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/grom-alex/dns-collector)

## Микросервисы

1. **[dns-collector](dns-collector/)** - сервис сбора и резолвинга DNS запросов
2. **[web-api](web-api/)** - веб-интерфейс для визуализации и анализа данных

## Возможности

### dns-collector
- UDP сервер для приема DNS запросов в JSON формате
- Хранение доменных имен и статистики в PostgreSQL
- Периодический резолвинг доменов в IP адреса (IPv4 и IPv6)
- Сбор статистики по запросам
- Настраиваемые параметры через конфигурационный файл

### web-api
- Веб-интерфейс для просмотра статистики DNS запросов
- Фильтрация по IP клиента, подсети, датам
- Просмотр доменов с зарезолвленными IP адресами
- Поиск доменов по регулярным выражениям
- **Экспорт данных в Excel (XLSX)** с полным форматированием
- **Экспорт списков для pfSense** с расширенной фильтрацией IPv4/IPv6
- **Исключение shared IP** для безопасной блокировки CDN/облачных сервисов
- **Интеграция threat intelligence** через дополнительные файлы IP
- REST API для интеграции с другими системами

## Быстрый старт

```bash
# Запуск всех сервисов (PostgreSQL, dns-collector, web-api)
docker-compose up -d

# Проверка статуса
docker-compose ps

# Просмотр логов
docker-compose logs -f

# Web-интерфейс доступен по адресу
http://localhost:8080

# Health-check API
curl http://localhost:8080/health
```

**Системные требования:**
- Docker и Docker Compose
- Порты 5353/udp (DNS collector), 8080 (Web UI), 5432 (PostgreSQL)
- Минимум 1GB RAM

Подробнее: [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md)

## Установка

```bash
# Клонировать репозиторий
git clone git@github.com:grom-alex/dns-collector.git
cd dns-collector

# Запустить с помощью Docker Compose (рекомендуется)
docker-compose up -d

# Или собрать локально
cd dns-collector
go mod download
go build -o dns-collector ./cmd/dns-collector
```

## Конфигурация

Пример файла `config.yaml`:

```yaml
server:
  udp_port: 5353

database:
  host: "postgres"       # Хост PostgreSQL
  port: 5432            # Порт PostgreSQL
  user: "dns_collector" # Пользователь БД
  password: "dns_collector_pass"  # Пароль БД
  database: "dns_collector"       # Название БД
  ssl_mode: "disable"   # Режим SSL (disable/require)

resolver:
  interval_seconds: 300  # Период резолвинга (5 минут)
  max_resolv: 10        # Максимальное количество резолвингов для домена
  timeout_seconds: 5    # Таймаут DNS запроса
  workers: 5           # Количество параллельных воркеров

logging:
  level: "info"  # Уровень логирования (debug, info, warn, error)
```

## Запуск

```bash
# С конфигурацией по умолчанию (config.yaml)
./dns-collector

# С указанием пути к конфигурации
./dns-collector -config /path/to/config.yaml
```

## Формат входящих сообщений

Программа принимает UDP сообщения в JSON формате:

```json
{
  "client_ip": "192.168.0.10",
  "domain": "google.com",
  "qtype": "AAAA",
  "rtype": "cache"
}
```

Поля:
- `client_ip` - IP адрес клиента, отправившего DNS запрос
- `domain` - доменное имя для резолвинга (обязательное)
- `qtype` - тип DNS запроса (пока не используется)
- `rtype` - откуда производился резолвинг (cache/dns)

## Тестирование

Отправка тестового запроса:

```bash
# Linux/macOS
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353

# Или с помощью Python
python3 test_client.py
```

## Структура базы данных

Система использует PostgreSQL для хранения всех данных.

**Таблица `domain`:**
- `id` - уникальный идентификатор (SERIAL PRIMARY KEY)
- `domain` - доменное имя (VARCHAR UNIQUE)
- `time_insert` - время вставки записи (TIMESTAMP)
- `resolv_count` - счетчик резолвингов (INTEGER)
- `max_resolv` - максимальное количество резолвингов (INTEGER)
- `last_resolv_time` - время последнего резолвинга (TIMESTAMP)

**Таблица `ip`:**
- `id` - уникальный идентификатор (SERIAL PRIMARY KEY)
- `domain_id` - связь с таблицей domain (INTEGER REFERENCES domain(id))
- `ip` - IP адрес (VARCHAR)
- `type` - тип адреса (VARCHAR: 'ipv4' или 'ipv6')
- `time` - время вставки/обновления (TIMESTAMP)

**Таблица `domain_stat`:**
- `id` - уникальный идентификатор (SERIAL PRIMARY KEY)
- `domain` - доменное имя (VARCHAR)
- `client_ip` - IP клиента (VARCHAR)
- `rtype` - тип резолвинга (VARCHAR)
- `timestamp` - время запроса (TIMESTAMP)

## Логика работы

1. UDP сервер принимает JSON сообщения с доменными именами
2. Каждый запрос сохраняется в таблицу `domain_stat` для статистики
3. Доменное имя добавляется в таблицу `domain` (если его там еще нет)
4. Периодически запускается задача резолвинга:
   - Выбираются домены с `resolv_count < max_resolv`
   - Для каждого домена выполняется DNS запрос
   - Полученные IP адреса сохраняются в таблицу `ip`
   - Обновляется счетчик `resolv_count` и время `last_resolv_time`
5. Резолвинг прекращается когда `resolv_count` достигает `max_resolv`

## Production Deployment

Для production окружения используйте конфигурацию из `deploy/production/`:

```bash
cd deploy/production

# Настройте переменные окружения
cp .env.example .env
nano .env

# Запустите сервисы
docker-compose up -d
```

**Особенности production конфигурации:**
- `pull_policy: always` - автоматическое обновление образов
- Ротация логов (max-size: 50m, max-file: 5)
- Увеличенные лимиты ресурсов
- SSL режим для PostgreSQL (опционально)
- Healthcheck для всех сервисов

Подробнее: [DEPLOYMENT.md](DEPLOYMENT.md)

## Документация

### Общая
- [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md) - быстрый старт с веб-интерфейсом
- [DEPLOYMENT.md](DEPLOYMENT.md) - production развертывание
- [SERVICES.md](SERVICES.md) - архитектура микросервисов
- [CHANGELOG.md](CHANGELOG.md) - история изменений

### DNS Collector
- [dns-collector/README.md](dns-collector/README.md) - обзор сервиса
- [dns-collector/INSTALL.md](dns-collector/INSTALL.md) - установка
- [dns-collector/QUICKSTART.md](dns-collector/QUICKSTART.md) - быстрый старт
- [dns-collector/ARCHITECTURE.md](dns-collector/ARCHITECTURE.md) - архитектура
- [dns-collector/DOCKER.md](dns-collector/DOCKER.md) - Docker развертывание

### Web API
- [web-api/README.md](web-api/README.md) - документация Web API
- [web-api/INSTALL.md](web-api/INSTALL.md) - установка и отладка web-api
- [web-api/FEATURES.md](web-api/FEATURES.md) - возможности API

## Возможности Web UI

### Статистика DNS запросов
- Фильтрация по IP клиента и подсети (с поддержкой CIDR)
- Фильтрация по диапазону дат
- Сортировка по любому полю
- Пагинация с корректным подсчетом результатов
- Применение фильтров по нажатию Enter

### Просмотр доменов
- Поиск по регулярным выражениям (PostgreSQL regex)
- Просмотр всех зарезолвленных IP адресов под строкой домена
- Компактная таблица IP адресов с цветовой кодировкой (IPv4/IPv6)
- Фильтрация по датам
- Пагинация отфильтрованных результатов

### Excel Экспорт
- Экспорт статистики DNS запросов в Excel (XLSX)
- Экспорт доменов с IP адресами (2 листа: Domains + IP Addresses)
- Полное форматирование: жирные заголовки, закрепление панелей, автофильтры
- Применение всех фильтров к экспортируемым данным
- Лимит 100,000 записей для безопасности
- Кнопки экспорта интегрированы в веб-интерфейс

### API Endpoints
- `GET /api/stats` - статистика DNS запросов
- `GET /api/domains` - список доменов
- `GET /api/domains/:id` - детали домена с IP адресами
- `GET /api/stats/export` - экспорт статистики в Excel
- `GET /api/domains/export` - экспорт доменов в Excel
- `GET /health` - health-check endpoint
