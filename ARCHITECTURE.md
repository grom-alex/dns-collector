# Архитектура DNS Collector

## Общая схема работы

```
┌─────────────────┐
│  DNS Clients    │
│ (UDP requests)  │
└────────┬────────┘
         │ JSON {"client_ip": ..., "domain": ..., ...}
         ▼
┌─────────────────────────────────────────┐
│         UDP Server (port 5353)          │
│  • Принимает JSON сообщения             │
│  • Валидация данных                     │
└────────┬────────────────────────────────┘
         │
         ├─────────────────┐
         ▼                 ▼
┌──────────────────┐  ┌────────────────────┐
│   domains.db     │  │    stats.db        │
│                  │  │                    │
│  ┌────────────┐  │  │  ┌──────────────┐  │
│  │  domain    │  │  │  │ domain_stat  │  │
│  │  table     │  │  │  │  table       │  │
│  └────────────┘  │  │  └──────────────┘  │
│  ┌────────────┐  │  │                    │
│  │   ip       │  │  │  • Каждый запрос   │
│  │   table    │  │  │  • Статистика      │
│  └────────────┘  │  └────────────────────┘
└──────────────────┘
         │
         │
         ▼
┌─────────────────────────────────────────┐
│      DNS Resolver (Worker Pool)         │
│  • Периодическая задача                 │
│  • Выбор доменов для резолвинга         │
│  • Параллельные DNS запросы             │
│  • Обновление IP таблицы                │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│         External DNS Servers            │
│         (система DNS)                   │
└─────────────────────────────────────────┘
```

## Компоненты системы

### 1. UDP Server (`internal/server/udp.go`)

**Назначение**: Прием DNS запросов от клиентов через UDP протокол

**Функции**:
- Прослушивание UDP порта
- Парсинг JSON сообщений
- Валидация входящих данных
- Асинхронная обработка сообщений
- Сохранение в базы данных

**Формат входящего сообщения**:
```json
{
  "client_ip": "192.168.0.10",
  "domain": "google.com",
  "qtype": "A",
  "rtype": "dns"
}
```

**Поток обработки**:
1. Получение UDP пакета
2. Десериализация JSON
3. Валидация (обязательное поле: domain)
4. Запись в `domain_stat` (статистика)
5. Вставка/получение записи в `domain`

### 2. Database Layer (`internal/database/database.go`)

**Назначение**: Работа с SQLite базами данных

#### Структура domains.db

**Таблица `domain`**:
```sql
CREATE TABLE domain (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    time_insert DATETIME NOT NULL,
    resolv_count INTEGER NOT NULL DEFAULT 0,
    max_resolv INTEGER NOT NULL,
    last_resolv_time DATETIME NOT NULL
);
```

- `id` - уникальный идентификатор
- `domain` - доменное имя (уникальное)
- `time_insert` - время первой вставки
- `resolv_count` - счетчик выполненных резолвингов
- `max_resolv` - максимальное количество резолвингов (из конфига)
- `last_resolv_time` - время последнего резолвинга

**Таблица `ip`**:
```sql
CREATE TABLE ip (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain_id INTEGER NOT NULL,
    ip TEXT NOT NULL,
    type TEXT NOT NULL,
    time DATETIME NOT NULL,
    UNIQUE(domain_id, ip),
    FOREIGN KEY(domain_id) REFERENCES domain(id)
);
```

- `id` - уникальный идентификатор
- `domain_id` - FK на таблицу domain
- `ip` - IP адрес
- `type` - тип адреса (ipv4/ipv6)
- `time` - время вставки/обновления
- **Уникальность**: пара (domain_id, ip)

#### Структура stats.db

**Таблица `domain_stat`**:
```sql
CREATE TABLE domain_stat (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype TEXT NOT NULL,
    timestamp DATETIME NOT NULL
);
```

- `id` - уникальный идентификатор
- `domain` - доменное имя из запроса
- `client_ip` - IP клиента
- `rtype` - тип резолвинга (cache/dns/etc)
- `timestamp` - время запроса

#### Основные операции

**InsertOrGetDomain**:
- INSERT OR IGNORE для атомарности
- Возвращает существующую или новую запись
- Устанавливает начальные значения из конфига

**GetDomainsToResolve**:
- Выбирает домены где `resolv_count < max_resolv`
- Сортировка по `last_resolv_time ASC` (старые первыми)
- Ограничение по количеству (batch)

**InsertOrUpdateIP**:
- INSERT ... ON CONFLICT DO UPDATE
- Обновляет время при повторной вставке
- Уникальность по паре (domain_id, ip)

**UpdateDomainResolvStats**:
- Инкремент `resolv_count`
- Обновление `last_resolv_time`

### 3. DNS Resolver (`internal/resolver/resolver.go`)

**Назначение**: Периодический резолвинг доменных имен в IP адреса

**Архитектура**:
- Периодический тикер (configurable interval)
- Пул воркеров для параллельной обработки
- Таймауты для DNS запросов
- Резолвинг IPv4 и IPv6

**Алгоритм работы**:

```
1. Тикер срабатывает каждые N секунд
   │
   ▼
2. Выбор доменов из БД (resolv_count < max_resolv)
   │
   ▼
3. Распределение по пулу воркеров
   │
   ▼
4. Для каждого домена (параллельно):
   ├─▶ DNS запрос IPv4 (LookupIP "ip4")
   ├─▶ DNS запрос IPv6 (LookupIP "ip6")
   ├─▶ Вставка/обновление IP в базе
   └─▶ Обновление счетчиков domain
   │
   ▼
5. Ожидание завершения всех воркеров
   │
   ▼
6. Завершение задачи, ожидание следующего тика
```

**Worker Pool**:
- Количество воркеров настраивается в конфиге
- Канал для распределения доменов
- WaitGroup для синхронизации

**DNS Resolution**:
```go
// IPv4
ipv4Addrs, err := resolver.LookupIP(ctx, "ip4", domain)

// IPv6
ipv6Addrs, err := resolver.LookupIP(ctx, "ip6", domain)
```

**Обработка результатов**:
- Даже при ошибке резолвинга инкрементируем счетчик
- Это предотвращает бесконечные попытки для несуществующих доменов
- При достижении max_resolv домен больше не обрабатывается

### 4. Configuration (`internal/config/config.go`)

**Назначение**: Загрузка и валидация конфигурации из YAML

**Параметры**:

```yaml
server:
  udp_port: 5353               # UDP порт сервера

database:
  domains_db: "domains.db"     # Путь к БД доменов
  stats_db: "stats.db"         # Путь к БД статистики

resolver:
  interval_seconds: 300        # Период резолвинга (сек)
  max_resolv: 10              # Max резолвингов на домен
  timeout_seconds: 5          # Таймаут DNS запроса
  workers: 5                  # Количество воркеров

logging:
  level: "info"               # Уровень логирования
```

**Валидация**:
- Проверка диапазона UDP порта (1-65535)
- Проверка положительности значений
- Установка значений по умолчанию

### 5. Main Application (`cmd/dns-collector/main.go`)

**Назначение**: Точка входа, инициализация и координация компонентов

**Последовательность запуска**:
1. Парсинг флагов командной строки
2. Загрузка конфигурации
3. Инициализация баз данных
4. Запуск UDP сервера
5. Запуск DNS resolver
6. Ожидание сигнала завершения
7. Graceful shutdown

**Graceful Shutdown**:
```go
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
<-sigCh
// Остановка компонентов в обратном порядке
```

## Потоки данных

### Поток 1: Прием DNS запроса

```
Client
  │
  │ UDP: {"client_ip":"...", "domain":"...", ...}
  ▼
UDP Server
  │
  ├─▶ Parse JSON
  ├─▶ Validate
  │
  ├─▶ INSERT INTO domain_stat (domain, client_ip, rtype, timestamp)
  │   (stats.db)
  │
  └─▶ INSERT OR IGNORE INTO domain (domain, time_insert, ...)
      (domains.db)
```

### Поток 2: DNS Резолвинг

```
Timer Tick (every N seconds)
  │
  ▼
SELECT domain FROM domain WHERE resolv_count < max_resolv
  │
  ▼
Worker Pool (parallel processing)
  │
  ├─▶ Worker 1: domain1 → DNS Lookup → INSERT/UPDATE ip
  ├─▶ Worker 2: domain2 → DNS Lookup → INSERT/UPDATE ip
  ├─▶ Worker 3: domain3 → DNS Lookup → INSERT/UPDATE ip
  └─▶ ...
  │
  ▼
For each domain:
  UPDATE domain SET resolv_count++, last_resolv_time=now
```

## Особенности реализации

### Конкурентность

1. **UDP Server**: каждое сообщение обрабатывается в отдельной горутине
2. **Resolver**: пул воркеров с каналами для распределения нагрузки
3. **Database**: SQLite с WAL mode для лучшей конкурентности

### Надежность

1. **INSERT OR IGNORE**: предотвращает дубликаты доменов
2. **ON CONFLICT DO UPDATE**: обновляет существующие IP
3. **Счетчик резолвингов**: предотвращает бесконечные попытки
4. **Таймауты**: ограничение времени DNS запросов

### Производительность

1. **Пул воркеров**: параллельная обработка доменов
2. **Индексы БД**: быстрый поиск по ключевым полям
3. **Batch processing**: обработка доменов пакетами
4. **Асинхронная обработка**: UDP сообщения не блокируют друг друга

## Масштабирование

### Вертикальное

- Увеличение количества воркеров (`workers`)
- Уменьшение интервала резолвинга (`interval_seconds`)
- Увеличение batch size в коде

### Горизонтальное

Для горизонтального масштабирования потребуется:
- Централизованная БД (PostgreSQL/MySQL вместо SQLite)
- Балансировщик для UDP трафика
- Координация между экземплярами (distributed locks)

## Мониторинг

### Логи

Программа логирует:
- Прием UDP сообщений
- Результаты DNS резолвинга
- Ошибки обработки
- Старт/стоп компонентов

### Метрики (в БД)

- Количество доменов
- Количество IP адресов
- Статистика запросов
- Прогресс резолвинга

Используйте `monitor.sh` для просмотра метрик.

## Безопасность

### Текущие меры

- Валидация входящих данных
- Ограничение размера UDP пакетов (4096 байт)
- Таймауты для внешних запросов

### Рекомендации

- Файрвол для ограничения доступа к UDP порту
- Rate limiting для предотвращения флуда
- Мониторинг размера БД
- Периодическая очистка старых записей статистики
