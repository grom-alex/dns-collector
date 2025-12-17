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
- Хранение доменных имен в SQLite
- Периодический резолвинг доменов в IP адреса (IPv4 и IPv6)
- Сбор статистики по запросам
- Настраиваемые параметры через конфигурационный файл

### web-api
- Веб-интерфейс для просмотра статистики DNS запросов
- Фильтрация по IP клиента, подсети, датам
- Просмотр доменов с зарезолвленными IP адресами
- Поиск доменов по регулярным выражениям
- REST API для интеграции с другими системами

## Быстрый старт

```bash
# Запуск всех сервисов
docker-compose up -d

# Web-интерфейс доступен по адресу
http://localhost:8080
```

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
  domains_db: "domains.db"
  stats_db: "stats.db"

resolver:
  interval_seconds: 300  # Период резолвинга (5 минут)
  max_resolv: 10        # Максимальное количество резолвингов для домена
  timeout_seconds: 5    # Таймаут DNS запроса
  workers: 5           # Количество параллельных воркеров

logging:
  level: "info"
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

### domains.db

**Таблица `domain`:**
- `id` - уникальный идентификатор
- `domain` - доменное имя (уникальное)
- `time_insert` - время вставки записи
- `resolv_count` - счетчик резолвингов
- `max_resolv` - максимальное количество резолвингов
- `last_resolv_time` - время последнего резолвинга

**Таблица `ip`:**
- `id` - уникальный идентификатор
- `domain_id` - связь с таблицей domain
- `ip` - IP адрес
- `type` - тип адреса (ipv4/ipv6)
- `time` - время вставки/обновления

### stats.db

**Таблица `domain_stat`:**
- `id` - уникальный идентификатор
- `domain` - доменное имя
- `client_ip` - IP клиента
- `rtype` - тип резолвинга
- `timestamp` - время запроса

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

## Документация

### Общая
- [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md) - быстрый старт с веб-интерфейсом
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

## Скриншоты

### Статистика DNS запросов
- Фильтрация по IP клиента и подсети
- Фильтрация по диапазону дат
- Сортировка по любому полю
- Пагинация

### Просмотр доменов
- Поиск по регулярным выражениям
- Просмотр всех зарезолвленных IP адресов
- Разделение IPv4/IPv6
- Фильтрация по датам
