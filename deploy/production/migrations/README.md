# Миграции базы данных

Этот каталог содержит SQL скрипты миграций PostgreSQL для проекта DNS Collector.

## Файлы миграций

- `001_initial_schema.sql` - Создание начальной схемы базы данных (таблицы)
- `002_add_indexes.sql` - Добавление индексов для производительности

## Запуск миграций

### Вариант 1: Ручное выполнение

Подключитесь к PostgreSQL и выполните миграции по порядку:

```bash
# Подключение к базе данных
psql -h localhost -U dns_collector -d dns_collector

# Выполнение миграций
\i migrations/001_initial_schema.sql
\i migrations/002_add_indexes.sql
```

### Вариант 2: Использование Docker

```bash
# Скопировать миграции в контейнер
docker cp migrations/ dns-collector-postgres:/tmp/

# Выполнить миграции
docker exec -i dns-collector-postgres psql -U dns_collector -d dns_collector < migrations/001_initial_schema.sql
docker exec -i dns-collector-postgres psql -U dns_collector -d dns_collector < migrations/002_add_indexes.sql
```

### Вариант 3: Автоматическая инициализация Docker

Разместите файлы миграций в каталоге `docker-entrypoint-initdb.d/` перед первым запуском:

```bash
mkdir -p ./postgres-init
cp migrations/*.sql ./postgres-init/

# Обновите docker-compose.yml:
volumes:
  - ./postgres-init:/docker-entrypoint-initdb.d
  - postgres_data:/var/lib/postgresql/data
```

Скрипты из `/docker-entrypoint-initdb.d/` выполняются автоматически при первом запуске контейнера (когда БД пустая).

## Проверка

После выполнения миграций проверьте схему:

```sql
-- Список таблиц
\dt

-- Проверка индексов
\di

-- Описание схемы
\d domain
\d ip
\d domain_stat
```

## Примечания по производительности

- Индексы создаются с `IF NOT EXISTS` для безопасного повторного запуска
- Частичные индексы используются для часто запрашиваемых свежих данных
- Составные индексы оптимизируют типичные шаблоны запросов
- Проверяйте производительность запросов с помощью `EXPLAIN ANALYZE`

## Обслуживание индексов

Мониторинг использования индексов:

```sql
-- Статистика использования индексов
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan as index_scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;

-- Поиск неиспользуемых индексов
SELECT
    schemaname,
    tablename,
    indexname
FROM pg_stat_user_indexes
WHERE idx_scan = 0
    AND indexname NOT LIKE '%_pkey';
```

## Устранение проблем

Если миграции не выполняются:

1. Проверьте логи PostgreSQL: `docker logs dns-collector-postgres`
2. Проверьте подключение: `docker exec -it dns-collector-postgres psql -U dns_collector -d dns_collector -c '\l'`
3. Проверьте существующую схему: `\dt` и `\di`
4. Для повторной инициализации удалите и создайте БД заново (ВНИМАНИЕ: потеря данных):
   ```sql
   DROP DATABASE dns_collector;
   CREATE DATABASE dns_collector;
   ```
