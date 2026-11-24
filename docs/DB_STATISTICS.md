# Статистика PostgreSQL: pg_stat_statements

**Статус:** включено на боевом стенде 192.168.0.15  
**Дата:** 2026-11-23, 00:35 +06 (2026-11-22 18:35 UTC)  
**PostgreSQL:** 16.11, extension `pg_stat_statements 1.10`

## Что сделано

1. В `postgresql.auto.conf` добавлены настройки:
   - `shared_preload_libraries = 'pg_stat_statements'`;
   - `track_io_timing = on`.
2. PostgreSQL перезапущен (контейнер `dns-collector-postgres`, простой ~1 с; сервисы остались доступны).
3. Создано расширение в БД `dns_collector`:
   ~~~sql
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
   ~~~
4. Статистика сброшена, чтобы начать чистый период:
   ~~~sql
   SELECT pg_stat_statements_reset();
   ~~~
5. Проверено: collector и web-api продолжили работу; `/health` web-api — `ok`, метрики collector — `ok`.

Текущие параметры:

| Параметр | Значение | Комментарий |
|---|---|---|
| `shared_preload_libraries` | `pg_stat_statements` | требует рестарта, уже применено |
| `pg_stat_statements.max` | 5 000 (default) | хватает; если `dealloc > 0` — увеличить и перезапустить |
| `pg_stat_statements.track` | `top` (default) | верхнеуровневые запросы; вложенные не нужны |
| `pg_stat_statements.track_utility` | `on` (default) | учитывать DDL/utility |
| `pg_stat_statements.save` | `on` (default) | статистика переживает рестарт |
| `track_io_timing` | `on` | тайминги чтения/записи в `pg_stat_io`/`pg_stat_wal` |

Сырые данные: [pg_stat_statements-initial.txt](baseline/2026-11-23/pg_stat_statements-initial.txt), [pg_stat_statements-2min.txt](baseline/2026-11-23/pg_stat_statements-2min.txt), [pg-stat-statements-settings.txt](baseline/2026-11-23/pg-stat-statements-settings.txt), [pg-stat-statements-config.txt](baseline/2026-11-23/pg-stat-statements-config.txt).

## Проверка

~~~sql
SHOW shared_preload_libraries;
SHOW track_io_timing;

SELECT extname, extversion
FROM pg_extension
WHERE extname = 'pg_stat_statements';

SELECT * FROM pg_stat_statements_info;
~~~

Ожидаемо:

- `shared_preload_libraries = pg_stat_statements`;
- `track_io_timing = on`;
- extension `1.10`;
- `pg_stat_statements_info.stats_reset` — время последнего сброса.

## Первый срез (60 секунд после включения)

Снят в 00:36–00:37 +06. Трафик в этот момент был ниже среднего (~82 пакета/мин), но резолвер уже выполнил батч, поэтому вклад виден.

| Запрос | Calls | Total, ms | Mean, ms | WAL, bytes | WAL/вызов |
|---|---:|---:|---:|---:|---:|
| `INSERT INTO ip ... ON CONFLICT DO UPDATE SET time` | 2 576 | 608,1 | 0,24 | 29 422 670 | **11,4 KB** |
| `UPDATE domain SET resolv_count = CASE ...` | 995 | 130,9 | 0,13 | 4 440 403 | 4,46 KB |
| `UPDATE domain SET last_seen = now()` | 82 | 27,0 | 0,33 | 806 088 | 9,8 KB |
| `INSERT INTO domain_stat ...` | 82 | 15,0 | 0,18 | 345 189 | 4,2 KB |
| `INSERT INTO domain ... ON CONFLICT DO NOTHING` | 82 | 40,9 | 0,50 | 425 | ~5 B |
| `SELECT COUNT(*) FROM ip` | 3 | 104,4 | **34,8** | 0 | — |
| `SELECT COUNT(*) FROM domain` | 3 | 18,2 | 6,06 | 0 | — |

Что это показывает:

1. **IP-upsert — главный источник WAL в выборке:** 2 576 вызовов за 60 с и 29,4 MB WAL. Это 11,4 KB WAL на один вызов; при 1,7 млн вызовов/сутки — десятки GB WAL. Подтверждает этап 1–2 плана.
2. **`UPDATE domain` на резолв** — второй источник: 4,46 KB WAL на вызов.
3. **`last_seen` на каждый пакет** — 9,8 KB WAL на вызов, хотя обновляется одно поле; при 400 тыс. пакетов/сутки это заметный вклад.
4. **`COUNT(*) FROM ip`** — 34,8 ms среднее на 400 тыс. строк; метрика `dns_db_ips_total` обновляется раз в 30 с. Не критично, но при росте таблицы станет хуже.
5. **`INSERT INTO domain` для известного домена** возвращает 0 строк, но всё равно выполняется на каждый пакет — кандидат на кэш `domain.id`.

Важно: это короткий срез с низким трафиком; он не заменяет 24-часовой профиль. Для устойчивых выводов используйте `total_exec_time`/`wal_bytes` за сутки.

### Контрольный срез через 2,5 минуты

Снят в 00:38 +06, [pg_stat_statements-2min.txt](baseline/2026-11-23/pg_stat_statements-2min.txt). Всего 23 нормализованных запроса, 4 618 вызовов, 35,9 MB WAL.

| Запрос | Calls | WAL, bytes | Total, ms |
|---|---:|---:|---:|
| `INSERT INTO ip ... ON CONFLICT DO UPDATE` | 2 578 | 29 424 146 | 608,4 |
| `UPDATE domain SET resolv_count = CASE ...` | 1 000 | 4 444 504 | 131,5 |
| `UPDATE domain SET last_seen = $1` | 253 | 1 097 827 | 58,3 |
| `INSERT INTO domain_stat ...` | 253 | 846 853 | 38,2 |
| `SELECT COUNT(*) FROM ip` | 5 | 24 134 | 173,2 |
| `INSERT INTO domain ... ON CONFLICT DO NOTHING` | 253 | 1 547 | 126,3 |

Выводы те же и более устойчивые: IP-upsert даёт **82% WAL** в выборке, `UPDATE domain` — ещё 12%; `last_seen` и `domain_stat` — по 2–3%. `COUNT(*) FROM ip` остаётся дорогим (34,6 ms среднее).

## Полезные запросы

### Топ по времени

~~~sql
SELECT
    calls,
    round(total_exec_time::numeric, 1) AS total_ms,
    round(mean_exec_time::numeric, 2) AS mean_ms,
    rows,
    round(100.0 * shared_blks_hit / NULLIF(shared_blks_hit + shared_blks_read, 0), 1) AS cache_hit_pct,
    left(query, 160) AS query
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
~~~

### Топ по WAL (ключевое для износа диска)

~~~sql
SELECT
    calls,
    wal_bytes,
    round(wal_bytes::numeric / NULLIF(calls, 0), 1) AS wal_per_call,
    round(total_exec_time::numeric, 1) AS total_ms,
    left(query, 160) AS query
FROM pg_stat_statements
ORDER BY wal_bytes DESC
LIMIT 20;
~~~

### Топ по числу вызовов

~~~sql
SELECT calls, round(mean_exec_time::numeric, 2) AS mean_ms,
       round(total_exec_time::numeric, 1) AS total_ms,
       rows, left(query, 160) AS query
FROM pg_stat_statements
ORDER BY calls DESC
LIMIT 20;
~~~

### Самые медленные в среднем (calls > 100)

~~~sql
SELECT calls, round(mean_exec_time::numeric, 2) AS mean_ms,
       round(total_exec_time::numeric, 1) AS total_ms,
       left(query, 160) AS query
FROM pg_stat_statements
WHERE calls > 100
ORDER BY mean_exec_time DESC
LIMIT 20;
~~~

### Сброс окна

~~~sql
SELECT pg_stat_statements_reset();
~~~

Сбрасывайте только перед новым измерением; иначе потеряете историю. `stats_reset` виден в `pg_stat_statements_info`.

### Переполнение

~~~sql
SELECT * FROM pg_stat_statements_info;
~~~

Если `dealloc > 0` — часть запросов вытеснена. Увеличьте `pg_stat_statements.max` (например, 10 000) и перезапустите PostgreSQL.

## Важные оговорки

- `pg_stat_statements` считает **нормализованные** запросы (литералы заменены на `$1`), поэтому один и тот же шаблон с разными доменами — одна строка.
- `total_exec_time` — время выполнения SQL, без времени ожидания блокировок/клиента; для полной картины смотрите `pg_stat_activity`/`pg_stat_io`.
- `track_io_timing = on` даёт тайминги, но добавляет небольшие накладные расходы на каждый I/O; для текущего профиля это приемлемо.
- `save = on` сохраняет статистику при штатном завершении; при `pg_stat_statements_reset()` или пересоздании volume она очищается.
- Расширение создаётся **в каждой БД отдельно**. Если появится вторая БД, выполните `CREATE EXTENSION` и там.
- Настройки `shared_preload_libraries`/`track_io_timing` лежат в `postgresql.auto.conf` внутри volume. При пересоздании volume (`docker compose down -v`) их и расширение нужно настраивать заново; compose-файлы репозитория уже содержат `command` для preload.

## Откат

Если статистика мешает (маловероятно) или нужно вернуть исходное состояние:

~~~sql
DROP EXTENSION IF EXISTS pg_stat_statements;
ALTER SYSTEM RESET shared_preload_libraries;
ALTER SYSTEM RESET track_io_timing;
SELECT pg_reload_conf();
~~~

Затем перезапустить PostgreSQL:

~~~bash
docker restart dns-collector-postgres
~~~

После отката `shared_preload_libraries` снова пустой, `track_io_timing` — `off`.

## Связанные документы

- [DB_BASELINE.md](DB_BASELINE.md) — срез «до оптимизации» (без `pg_stat_statements`).
- [OPTIMIZATION_PLAN.md](OPTIMIZATION_PLAN.md) — этап 0 и дальнейшие шаги.
- [DB_MONITORING.md](DB_MONITORING.md) — 24-часовой сбор и гайд по анализу.
- [baseline/2026-11-23](baseline/2026-11-23/) — сырые данные.
