# DNS Collector - Краткая сводка проекта

## Что реализовано

✅ **Полнофункциональная программа на Go** для сбора DNS имен и резолвинга их в IP адреса

## Основные компоненты

### 1. UDP Сервер
- Прием JSON сообщений через UDP протокол
- Порт настраивается в конфигурации
- Асинхронная обработка запросов
- Формат: `{"client_ip": "...", "domain": "...", "qtype": "...", "rtype": "..."}`

### 2. База данных (SQLite)

**domains.db:**
- Таблица `domain` - хранение доменных имен
  - Уникальность по полю `domain`
  - Автоинкремент `id`
  - Счетчик резолвингов `resolv_count`
  - Максимум резолвингов `max_resolv` (из конфига)
  - Временные метки

- Таблица `ip` - хранение IP адресов
  - Связь с доменами через `domain_id`
  - Уникальность пары (domain_id, ip)
  - Тип адреса (ipv4/ipv6)
  - Временные метки

**stats.db:**
- Таблица `domain_stat` - статистика запросов
  - Каждый UDP запрос = новая запись
  - Сохранение domain, client_ip, rtype, timestamp

### 3. DNS Resolver
- Периодическая задача (интервал настраивается)
- Выбор доменов с условием: `resolv_count < max_resolv`
- Пул воркеров для параллельного резолвинга
- Резолвинг IPv4 и IPv6 адресов
- Автоматическое обновление таблиц:
  - INSERT/UPDATE в таблицу `ip`
  - Инкремент `resolv_count` и обновление `last_resolv_time` в `domain`

### 4. Конфигурация
- YAML файл с настройками
- Все параметры настраиваемые:
  - UDP порт
  - Пути к БД
  - Интервал резолвинга
  - Максимум резолвингов
  - Таймауты
  - Количество воркеров

## Структура проекта

```
dns-collector/
├── cmd/
│   └── dns-collector/
│       └── main.go                 # Точка входа
├── internal/
│   ├── config/
│   │   └── config.go               # Загрузка конфигурации
│   ├── database/
│   │   └── database.go             # Работа с SQLite
│   ├── resolver/
│   │   └── resolver.go             # DNS резолвер
│   └── server/
│       └── udp.go                  # UDP сервер
├── config.yaml                      # Конфигурация
├── go.mod                          # Go модули
├── Makefile                        # Команды сборки
├── Dockerfile                      # Docker образ
├── docker-compose.yml              # Docker Compose
├── dns-collector.service           # Systemd service
├── test_client.py                  # Тестовый клиент
├── monitor.sh                      # Скрипт мониторинга
├── queries.sql                     # SQL запросы для анализа
├── README.md                       # Основная документация
├── INSTALL.md                      # Инструкция по установке
├── DOCKER.md                       # Docker инструкции
└── ARCHITECTURE.md                 # Архитектура системы
```

## Файлы проекта

### Исходный код Go
- [cmd/dns-collector/main.go](cmd/dns-collector/main.go) - главный файл
- [internal/config/config.go](internal/config/config.go) - конфигурация
- [internal/database/database.go](internal/database/database.go) - база данных
- [internal/server/udp.go](internal/server/udp.go) - UDP сервер
- [internal/resolver/resolver.go](internal/resolver/resolver.go) - DNS resolver

### Конфигурация и сборка
- [config.yaml](config.yaml) - файл конфигурации
- [go.mod](go.mod) - зависимости Go
- [Makefile](Makefile) - команды сборки и запуска

### Развертывание
- [Dockerfile](Dockerfile) - Docker образ
- [docker-compose.yml](docker-compose.yml) - Docker Compose
- [dns-collector.service](dns-collector.service) - systemd сервис
- [.dockerignore](.dockerignore) - исключения для Docker

### Тестирование и мониторинг
- [test_client.py](test_client.py) - Python клиент для тестирования
- [monitor.sh](monitor.sh) - скрипт мониторинга статистики
- [queries.sql](queries.sql) - полезные SQL запросы

### Документация
- [README.md](README.md) - основная документация
- [INSTALL.md](INSTALL.md) - установка и запуск
- [DOCKER.md](DOCKER.md) - Docker развертывание
- [ARCHITECTURE.md](ARCHITECTURE.md) - архитектура системы
- [SUMMARY.md](SUMMARY.md) - этот файл

## Быстрый старт

### Вариант 1: Нативная сборка

```bash
# Установка зависимостей
go mod download

# Сборка
make build

# Запуск
./build/dns-collector
```

### Вариант 2: Docker

```bash
# Сборка и запуск
docker-compose up -d

# Просмотр логов
docker-compose logs -f
```

### Тестирование

```bash
# Python клиент
python3 test_client.py

# Или вручную через netcat
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353
```

### Мониторинг

```bash
# Скрипт мониторинга
./monitor.sh

# Или прямые SQL запросы
sqlite3 domains.db "SELECT COUNT(*) FROM domain;"
sqlite3 stats.db "SELECT COUNT(*) FROM domain_stat;"
```

## Основные особенности

### Реализованные требования

✅ UDP сервер с настраиваемым портом
✅ Прием данных в формате JSON
✅ Таблица `domain` со всеми полями и логикой
✅ Таблица `ip` с уникальностью (domain_id, ip)
✅ Таблица `domain_stat` для статистики
✅ Периодический резолвинг (настраиваемый период)
✅ Условие выборки: resolv_count < max_resolv
✅ DNS запросы для IPv4 и IPv6
✅ Инкремент resolv_count после резолвинга
✅ Обновление last_resolv_time
✅ Раздельные базы данных (domains.db, stats.db)

### Дополнительные возможности

✅ Пул воркеров для параллельного резолвинга
✅ Graceful shutdown
✅ Docker поддержка
✅ Systemd интеграция
✅ Скрипты мониторинга
✅ Готовые SQL запросы для анализа
✅ Тестовый клиент
✅ Подробная документация

## Конфигурация

```yaml
server:
  udp_port: 5353                    # UDP порт

database:
  domains_db: "domains.db"          # БД доменов
  stats_db: "stats.db"              # БД статистики

resolver:
  interval_seconds: 300             # Период резолвинга (5 мин)
  max_resolv: 10                    # Макс. резолвингов
  timeout_seconds: 5                # Таймаут DNS запроса
  workers: 5                        # Кол-во воркеров

logging:
  level: "info"                     # Уровень логов
```

## Архитектура

```
UDP Client → UDP Server → [ domain_stat (stats.db) ]
                        ↓
                     domain (domains.db)
                        ↑
DNS Resolver (periodic) → ip (domains.db)
     ↓
External DNS
```

## Производительность

- **Конкурентность**: пул воркеров для параллельной обработки
- **Асинхронность**: UDP запросы обрабатываются в горутинах
- **Индексы БД**: оптимизация запросов
- **Настройка**: количество воркеров и интервал резолвинга

## Безопасность

- Валидация входящих данных
- Таймауты для внешних запросов
- Ограничение размера UDP пакетов
- Graceful shutdown
- Возможность запуска от непривилегированного пользователя

## Масштабирование

- Вертикальное: увеличение воркеров и частоты резолвинга
- Горизонтальное: возможна миграция на PostgreSQL для множественных экземпляров

## Мониторинг и обслуживание

- Логирование всех операций
- Скрипт мониторинга (`monitor.sh`)
- Готовые SQL запросы (`queries.sql`)
- Health checks для Docker
- Systemd интеграция для автозапуска

## Что дальше?

### Рекомендуемые улучшения

1. **Метрики**: добавить Prometheus метрики
2. **API**: HTTP API для управления и мониторинга
3. **Web UI**: веб-интерфейс для визуализации
4. **Rate Limiting**: защита от флуда
5. **Очистка**: автоматическая очистка старых записей
6. **Уведомления**: алерты при проблемах
7. **Кластеризация**: поддержка нескольких инстансов

### Возможные расширения

- Поддержка TCP наряду с UDP
- Экспорт данных в различных форматах
- Интеграция с SIEM системами
- GeoIP определение для IP адресов
- Обнаружение аномалий в DNS трафике

## Зависимости

- Go 1.21+
- SQLite3
- github.com/mattn/go-sqlite3
- gopkg.in/yaml.v3

## Лицензия

Проект создан как пример реализации. Вы можете использовать его свободно.

## Поддержка

Для вопросов и предложений:
- Изучите документацию в [README.md](README.md)
- Проверьте [ARCHITECTURE.md](ARCHITECTURE.md) для понимания внутреннего устройства
- Используйте [INSTALL.md](INSTALL.md) для развертывания
- Смотрите [DOCKER.md](DOCKER.md) для контейнеризации

---

**Статус**: ✅ Полностью готов к использованию
**Версия**: 1.0.0
**Дата**: 2025-12-14
