# План устранения недостатков и оптимизации PostgreSQL

**Статус:** к исполнению  
**Дата:** 2026-11-22  
**Область:** dns-collector, web-api, PostgreSQL 16, PVE/ZFS  
**Приоритет №1:** снижение объёма записи в PostgreSQL (износ NVMe)

---

## 0. Резюме

Диск изнашивается в основном не из-за WAL, а из-за огромного числа **UPDATE** и **DELETE**, которые PostgreSQL превращает в грязные страницы, а checkpointer/bgwriter сбрасывают на диск. По baseline 2026-11-23 буферы дают ~49,9 ГБ/сутки, WAL — ~21,2 ГБ/сутки. Основные виновники:

1. `ip`: `INSERT ... ON CONFLICT DO UPDATE SET time = now()` на каждый IP при **каждом** резолве, даже если IP не изменился — 123,3 млн UPDATE, HOT 0%.
2. `domain`: `UPDATE resolv_count / last_resolv_time / last_seen` на каждый пакет и каждый резолв, причём индексы стоят на обновляемых столбцах — 82,8 млн UPDATE, HOT 0,03%.
3. `domain_stat`: вставка на каждый запрос + массовый `DELETE` по retention — 33,9 млн INSERT / 32,9 млн DELETE, 5,1 ГБ на 3 млн строк.
4. Слабые настройки PostgreSQL (`shared_buffers = 128 МБ`) и ZFS `volblocksize = 16K` при странице 8K — множители ×2…×3 к физической записи.

**Порядок лечения:** сначала убрать лишние записи в коде, затем убрать индексы, мешающие HOT, затем партиционировать статистику, затем тюнинг PostgreSQL, затем ZFS. Тюнинг сам по себе не компенсирует миллионы лишних UPDATE в сутки.

**Целевой эффект:** снизить запись в пул с ~2,6 МБ/с до 0,4–0,7 МБ/с, годовую запись с ~72 ТБ до 10–20 ТБ, износ с ~13%/год до ~2–3%/год. Цифры — оценка, проверяются на этапе 0 и после каждого этапа.

### 0.1. Первые 48 часов (минимальный набор)

Если нет времени на весь план, начните с этого — это самые дешёвые и самые эффективные шаги:

1. Снять бэкап и включить `pg_stat_statements`/`track_io_timing` (этап 0).
2. Убрать `log.Printf` на каждый UDP-пакет (или перевести в debug) — меньше I/O и логов.
3. Добавить `WHERE ip.time < now() - interval '1 day'` в `ON CONFLICT DO UPDATE` — без изменения схемы убирает большинство перезаписей `ip`.
4. Троттлинг `last_seen`: обновлять не чаще раза в 5 минут на домен (этап 1).
5. Удалить дублирующие индексы `idx_ip_time`, `idx_ip_domain_id`, `idx_ip_type`, `idx_domain_name`; поставить `fillfactor = 85` на `domain` и `ip` (этап 2).
6. Поднять `shared_buffers` до 4 ГБ и `checkpoint_timeout` до 15 минут (этап 4).
7. Замерить через 24 часа: `n_tup_upd`, `n_tup_hot_upd`, `pg_stat_bgwriter`, `zpool iostat`.

Ожидаемо: снижение записи в 2–4× без партиционирования и ZFS. Полный эффект — после этапов 3–6.

---

## 1. Исходная картина

### 1.1. Что показал отчёт

| Показатель | Значение | Комментарий |
|---|---:|---|
| Записано на NVMe | 72,75 ТБ за 8 879 ч | Kingston KC3000 1 ТБ, mirror |
| Текущий поток | ~2,6 МБ/с | ~224 ГБ/сутки, ~80 ТБ/год |
| Износ | 13% за год | при 800 TBW — ~7,5 лет до 100% |
| PostgreSQL | ~900 КБ/с | backend 476 + bgwriter 248 + checkpointer 182 КБ/с |
| WAL | 21,2 ГБ/сутки (по baseline) | существенно, но меньше буферов; включить `wal_compression` |
| `domain` | 15 572 строки, 82,8 млн UPDATE | ~5 300 перезаписей на строку |
| `domain` HOT | 22 379 из 82,8 млн (0,03%) | индекс на обновляемом столбце |
| `ip` | 123,3 млн UPDATE, HOT 0 | то же, но без HOT вообще |
| `domain_stat` | 33,9 млн INSERT / 32,9 млн DELETE | 5,1 ГБ на 3 млн строк |
| Autovacuum по `domain` | 22 093 прохода за 68 суток | ~325 проходов/сутки |
| Буферы checkpointer | 392 млн × 8 КБ = 3,1 ТБ / 68 суток | 46 ГБ/сутки |
| `shared_buffers` | 128 МБ | дефолт на VM с 16 ГБ |
| ZFS | `volblocksize = 16K` | каждая 8K-страница перезаписывает 16K-блок |

### 1.2. Главный вывод

Проблема не в объёме данных (15 тыс. доменов — это мало), а в **частоте и характере записи**. Одна и та же строка `ip` переписывается десятки раз в сутки, хотя её значение не меняется. Одна и та же строка `domain` обновляется на каждый DNS-пакет. Индексы на обновляемых столбцах запрещают HOT, поэтому каждая такая запись — это новая версия строки плюс обновление всех индексных записей, то есть несколько грязных 8K-страниц.

### 1.3. Фактический срез 2026-11-23

Снят read-only baseline с боевого стенда 192.168.0.15. Полный отчёт и сырые данные: [DB_BASELINE.md](DB_BASELINE.md), [baseline/2026-11-23](baseline/2026-11-23/).

| Показатель | Факт на 2026-11-23 |
|---|---:|
| Размер БД | 7 755 MB |
| Доля индексов в БД | **93,3%** (7 237 MB) |
| `domain_stat` | 5 090 MB, из них индексы 4 798 MB |
| `ip` | 1 466 MB, из них индексы 1 363 MB |
| `domain` | 1 192 MB на 16 279 строк, из них индексы 1 076 MB |
| `domain` UPDATE | 1,14 млн/сутки, HOT **0,021%** |
| `ip` UPDATE | 1,71 млн/сутки, HOT **0%** |
| `domain_stat` INSERT / DELETE | 0,42 / 0,41 млн/сутки |
| Буферы PG (checkpointer+bgwriter+backend) | **49,9 GB/сутки** |
| WAL | **21,2 GB/сутки** |
| Запись `/dev/sdb1` | **89,1 GB/сутки** (1,03 MB/s в среднем; 0,85 MB/s на 5-мин срезе) |
| `checkpoint_write_time` | **89,7%** календарного времени |
| Autovacuum | 280 проходов/сутки по `domain`; 16,1% записи PG; 92,7% чтения |
| Логи collector | 122 MB/сутки, 1,28 млн строк/сутки |
| IPv6-lookup с ошибкой | **71%** |
| Конфиг резолвера | интервал 120 с, 100 workers, батч 1 000 доменов |

Это подтверждает выводы отчёта и добавляет два важных факта: **93,3% БД — индексы** (кандидаты на удаление/замену — до 5,8 GB), а **autovacuum — крупный писатель и главный читатель** (16,1% записи, 92,7% чтения). Поэтому в плане приоритет отдан не только снижению UPDATE, но и чистке индексов/партиционированию.

Первый query-level срез после включения `pg_stat_statements` ([DB_STATISTICS.md](DB_STATISTICS.md)): один вызов `INSERT INTO ip ... ON CONFLICT DO UPDATE` генерирует **11,4 KB WAL**, `UPDATE domain` — 4,5 KB, `last_seen` — 9,8 KB. Это прямое подтверждение того, что основной объём WAL создают не «редкие тяжёлые» запросы, а массовые UPDATE.

24-часовой профиль (286 срезов) подтвердил это на устойчивом окне: **1,77 млн IP-upsert/сутки дают 14,28 GB WAL (69%)**, 1,13 млн `domain` UPDATE — 2,07 GB, 412 тыс. `domain_stat` INSERT — 1,83 GB. Полный отчёт: [DB_MONITORING_REPORT.md](DB_MONITORING_REPORT.md).

---

## 2. Диагноз: откуда берутся записи

### 2.1. Карта горячего пути

| Код | SQL-операция | Частота | Почему плохо |
|---|---|---|---|
| `dns-collector/internal/server/udp.go:126` | `INSERT INTO domain_stat` | каждый UDP-пакет | отдельный round-trip и отдельная страница на запись |
| `dns-collector/internal/server/udp.go:131` | `INSERT ... ON CONFLICT DO NOTHING` + `SELECT` | каждый пакет | для известного домена — лишняя запись/чтение; можно кэшировать ID |
| `dns-collector/internal/server/udp.go:138` | `UPDATE domain SET last_seen` | каждый пакет | строка `domain` и индекс `idx_domain_last_seen` переписываются постоянно |
| `dns-collector/internal/server/udp.go:123` | `log.Printf` | каждый пакет | не диск, но I/O и размер логов; при 500 rps — десятки ГБ/сутки |
| `dns-collector/internal/resolver/resolver.go:171,198` | `INSERT INTO ip ... ON CONFLICT DO UPDATE SET time = now()` | каждый IP при каждом резолве | даже неизменный IP получает новую версию строки; `time` индексирован → HOT невозможен |
| `dns-collector/internal/resolver/resolver.go:210` | `UPDATE domain SET resolv_count = ..., last_resolv_time = now()` | каждый домен при каждом резолве | индексы на `resolv_count` и `last_resolv_time` → HOT невозможен |
| `dns-collector/internal/cleanup/cleanup.go:78` | `DELETE FROM domain_stat WHERE timestamp < ...` | раз в сутки | массовые DELETE → dead tuples, bloat, autovacuum, WAL |
| `dns-collector/internal/cleanup/cleanup.go:92` | `DELETE FROM ip WHERE time < ...` | раз в сутки | то же, но меньше |
| `dns-collector/internal/cleanup/cleanup.go:107` | `DELETE FROM domain WHERE last_seen < ...` | раз в сутки | каскад по `ip` |

### 2.2. Индексы, которые убивают HOT

HOT-update возможен, только если **ни один индексируемый столбец не изменился**. Сейчас:

**`domain`:**

| Индекс | Столбцы | Мешает HOT? | Решение |
|---|---|---|---|
| PK `domain_pkey` | `id` | нет | оставить |
| `domain_domain_key` | `domain` | нет | оставить |
| `idx_domain_resolv` | `(resolv_count, max_resolv)` | **да** | удалить после переписывания запроса резолвера |
| `idx_domain_last_seen` | `(last_seen)` | **да** | удалить; 15 тыс. строк сканируются seq scan за миллисекунды |
| `idx_domain_resolv_count` | `(resolv_count) WHERE ...` | **да** | удалить |
| `idx_domain_last_resolv_time` | `(last_resolv_time)` | **да** | удалить или вынести планировщик в отдельную таблицу |
| `idx_domain_resolv_lookup` | `(resolv_count, last_resolv_time) WHERE ...` | **да** | удалить |
| `idx_domain_time_insert` | `(time_insert)` | нет | можно оставить |
| `idx_domain_name` | `(domain)` | нет | **удалить** — дублирует уникальный индекс |

**`ip`:**

| Индекс | Столбцы | Мешает HOT? | Решение |
|---|---|---|---|
| PK `ip_pkey` | `id` | нет | оставить |
| `ip_domain_id_ip_key` | `(domain_id, ip)` | нет (ip не меняется) | оставить, он же обслуживает поиск по `domain_id` |
| `idx_ip_domain_id` | `(domain_id)` | нет | **удалить** — дублирует префикс уникального индекса |
| `idx_ip_type` | `(type)` | **да** при обновлении type | удалить, низкая селективность |
| `idx_ip_time_cleanup` | `(time)` | **да** при `time = now()` | оставить только если `time` обновляется редко (раз в сутки), иначе заменить на BRIN/партицию |
| `idx_ip_time` | `(time)` | **да** | **удалить** — дубль `idx_ip_time_cleanup` |
| `idx_ip_address` | `(ip)` | нет (ip не меняется) | оставить для shared-IP запросов |
| `idx_ip_domain_type` | `(domain_id, type)` | **да** при обновлении type | удалить, уникальный индекс уже покрывает `domain_id` |

**`domain_stat`:** 6 индексов на таблицу, каждый INSERT пишет 7 индексных записей (включая PK). Для партиционированной append-only таблицы достаточно 2–3 индексов на партицию.

### 2.3. Почему настройки PostgreSQL не спасают

`shared_buffers = 128 МБ` означает, что грязные страницы вытесняются раньше, чем до них дойдёт checkpointer, и потом переписываются снова. `checkpoint_timeout` и `max_wal_size` по умолчанию малы, `jit = on` (из миграции 000003) для OLTP вреден, autovacuum на `domain` крутится почти непрерывно. Но всё это — множители к основной проблеме: по baseline 2026-11-23 это 1,14 млн `domain` + 1,71 млн `ip` UPDATE в сутки. Тюнинг даст 1,2–2×, а не 10×.

### 2.4. ZFS

`volblocksize = 16K` на zvol при странице PostgreSQL 8K: каждая случайная 8K-запись читает-модифицирует-пишет 16K-блок. Это ровно ×2 к физической записи. Плюс возможная компрессия и RAIDZ/mirror-накладные. Пока zvol не пересоздан, физическую запись можно уменьшить только сокращением логической.

---

## 3. Цели и KPI

| KPI | Сейчас | Цель | Как измерять |
|---|---:|---:|---|
| Логическая запись PostgreSQL | ~71 ГБ/сутки (49,9 буферы + 21,2 WAL) | ≤15 ГБ/сутки | `pg_stat_bgwriter`, `pg_stat_wal`, `pg_stat_statements` |
| `ip` UPDATE | 1,71 млн/сутки | ≤0,1 млн/сутки | `pg_stat_user_tables.n_tup_upd` |
| `domain` UPDATE | 1,14 млн/сутки | ≤0,2 млн/сутки | то же |
| HOT-доля `domain` | 0,021% | ≥90% | `n_tup_hot_upd / n_tup_upd` |
| `ip` HOT | 0% | по возможности; важнее абсолютное снижение | `n_tup_hot_upd` |
| `domain_stat` DELETE | 0,41 млн/сутки | 0 | DROP PARTITION вместо DELETE |
| Размер `domain_stat` | 5,09 ГБ на 3,1 млн строк | ≤1,5 ГБ | `pg_total_relation_size` |
| Autovacuum по `domain` | ~280/сутки | ≤50/сутки | `pg_stat_user_tables.autovacuum_count` |
| Запись в пул | ~2,6 МБ/с (отчёт); 1,03 МБ/с в VM (baseline) | ≤0,6 МБ/с | `zpool iostat -v 1`, `/proc/diskstats` |
| Годовая запись | ~72 ТБ | 10–20 ТБ | PVE/ZFS counters |
| Износ NVMe | 13%/год | ≤3%/год | `smartctl` |
| p95 `/api/stats` | не измерено | ≤500 мс | Prometheus |
| Экспорт 100 тыс. строк | риск OOM | без OOM, ≤60 с | нагрузочный тест |

---

## 4. Порядок работ

1. **Этап 0. База и измерения.** Бэкап, `pg_stat_statements`, фиксация baseline, feature flags.
2. **Этап 1. Убрать лишние записи в приложении.** Кэш доменов, батчинг статистики, троттлинг `last_seen`, IP-upsert только при изменении, убрать лог на каждый пакет. Это даёт основной эффект и не требует изменения схемы.
3. **Этап 2. Индексы и HOT.** Удалить индексы с обновляемых столбцов, поставить `fillfactor`, переписать запрос выбора доменов. Проверить рост HOT.
4. **Этап 3. Партиционирование `domain_stat`.** Убрать массовые DELETE, перейти на DROP PARTITION, уменьшить число индексов.
5. **Этап 4. Тюнинг PostgreSQL.** `shared_buffers`, checkpoints, autovacuum, `jit`.
6. **Этап 5. ZFS/VM.** Пересоздать zvol с `volblocksize = 8K` (или подтвердить, что 16K остаётся осознанным решением).
7. **Этап 6. Планировщик резолвера.** Не резолвить всё подряд; backoff, только активные домены, single-flight.
8. **Этап 7. web-api и экспорт.** Стриминг Excel, чанки `IN`, лимиты, таймауты, healthcheck.
9. **Этап 8. Безопасность и качество.** Аутентификация, UDP allowlist, тесты, CI.
10. **Этап 9. Наблюдаемость и регламенты.** Метрики записи, алерты, runbook.

Каждый этап заканчивается замером и записью «до/после» в этот документ. Если этап не дал ожидаемого эффекта — не переходить к следующему, а разбираться.

---

## 5. Этапы

### Этап 0. База, бэкап, измерения (0,5 дня)

- [ ] Снять бэкап: `pg_dump -Fc -f dns_collector.dump` (или `pg_basebackup`).
- [x] Включить `pg_stat_statements` (`shared_preload_libraries`) и `track_io_timing = on` — сделано 2026-11-23 00:35, см. [DB_STATISTICS.md](DB_STATISTICS.md).
- [x] Зафиксировать baseline на 2026-11-23 (сделано): [DB_BASELINE.md](DB_BASELINE.md).
- [x] Снять 24-часовой профиль (286 срезов, 0 ошибок): [DB_MONITORING_REPORT.md](DB_MONITORING_REPORT.md).
  - `pg_stat_user_tables`, `pg_stat_bgwriter`, `pg_stat_wal`, `pg_stat_io`, `pg_stat_user_indexes`;
  - `pg_stat_statements` — включён **после** baseline; первый 60-секундный срез и запросы — в [DB_STATISTICS.md](DB_STATISTICS.md);
  - `zpool iostat`/`smartctl` — **не сняты**: из VM не видны, нужен доступ к PVE.
- [ ] Ввести feature flags (env): `DB_BATCH_ENABLED`, `DB_LAST_SEEN_FLUSH_SEC`, `DB_IP_REFRESH_HOURS`, `RESOLVER_MAX_BACKOFF`.
- [ ] Убедиться, что миграции можно откатить, а приложение — откатить на предыдущий образ.

**Критерий:** есть дамп, baseline и возможность сравнить «до/после» по одним и тем же запросам.

### Этап 1. Горячий путь приложения (3 дня)

**Статус:** реализовано в PR #17, выкачено в 2.9.0. 24-часовой замер: WAL 20,83 → **7,52 GB/сутки**, запись `sdb1` 89,08 → **36,60 GB/сутки**, IP UPDATE 1,72 млн → **108 тыс.** Подробности — [DB_MONITORING_REPORT_2.9.0.md](DB_MONITORING_REPORT_2.9.0.md).

Цель — убрать 80–90% лишних записей без изменения схемы.

- [ ] **Кэш `domain.id`.** LRU-кэш `domain -> id` на 100 тыс. записей с TTL 24 ч. При попадании — не делать `INSERT ... ON CONFLICT` и `SELECT`.
- [ ] **Батчинг `domain_stat`.** Канал + горутина-писатель: батч 500 строк или 1 с, `COPY`/multi-row INSERT.
- [ ] **Троттлинг `last_seen`.** In-memory `map[domainID]time.Time`, сброс раз в 60 с одним `UPDATE ... FROM unnest(...)` с условием «не чаще раза в 5 минут на домен».
- [ ] **IP-upsert только при изменении.** Сравнивать набор IP с текущим (кэш/один SELECT на домен), вставлять только новые, обновлять `last_seen` не чаще раза в сутки, удалять выпавшие IP.
- [ ] **Убрать `log.Printf` на каждый пакет** или перевести в debug с sampling.
- [ ] **Ограничить горутины.** Пул воркеров + буферизованный канал вместо безлимитного `go handleMessage`; при переполнении — метрика и drop/backpressure.
- [ ] **Graceful flush** очередей при остановке.

**Критерий:** `n_tup_upd` по `ip` упал ≥10×, по `domain` — ≥3×; `domain_stat` вставляется батчами; при остановке ничего не теряется.

### Этап 2. Индексы, fillfactor, запрос резолвера (2 дня)

- [ ] Удалить индексы из таблицы выше: `idx_domain_resolv`, `idx_domain_last_seen`, `idx_domain_resolv_count`, `idx_domain_last_resolv_time`, `idx_domain_resolv_lookup`, `idx_ip_domain_id`, `idx_ip_type`, `idx_ip_domain_type`.
- [ ] Переписать `GetDomainsToResolve`: убрать OR-ветку с недостижимым cooldown (см. `resolver.go`) либо заменить её на `next_resolve_at`; сортировка по `last_resolv_time` без индекса допустима на 15 тыс. строк, но лучше сразу перейти на `domain_schedule` (этап 6).
- [ ] `ALTER TABLE domain SET (fillfactor = 85)`; `ALTER TABLE ip SET (fillfactor = 85)`.
- [ ] `domain_stat`: удалить лишние индексы (`rtype`, отдельные `client_ip` и `domain`, `DESC`-дубликаты), оставить `(timestamp)`, `(client_ip, timestamp)`, `(domain, timestamp)`.
- [ ] Проверить `EXPLAIN (ANALYZE, BUFFERS)` для запросов web-api: не появились ли seq scan на больших таблицах.

**Критерий:** для `domain` — `n_tup_hot_upd / n_tup_upd ≥ 0,9`; для `ip` — суммарные UPDATE ≤0,1 млн/сутки (если остаются суточные обновления `time`, HOT может быть низким — это допустимо); размер индексов уменьшился; p95 API не вырос.

### Этап 3. Партиционирование `domain_stat` (3 дня)

- [ ] Создать партиционированную таблицу по месяцам (или дням) с `PRIMARY KEY (id, timestamp)`.
- [ ] Перенести данные батчами, переключить имя, сохранить старую таблицу на 1 релиз.
- [ ] Настроить создание партиций (приложение при старте или `pg_partman`).
- [ ] Изменить retention: `DROP TABLE` просроченной партиции вместо `DELETE`; `DELETE` оставить как fallback.
- [ ] Для партиций: `fillfactor = 100`, минимум индексов, `autovacuum` не отключать (нужен analyze), но dead tuples не будет.
- [ ] Обновить `cleanup.go` и метрики (`CleanupStatsDeleted` → количество удалённых партиций/строк).

**Критерий:** `n_tup_del` по `domain_stat` = 0; размер таблицы ≤1,5 ГБ; запросы по датам используют pruning.

### Этап 4. Тюнинг PostgreSQL (1 день)

Ориентир для VM 16 ГБ (25% под `shared_buffers`):

| Параметр | Сейчас | Предлагается | Зачем |
|---|---:|---:|---|
| `shared_buffers` | 128 МБ | 4 ГБ | меньше вытеснений и повторных записей |
| `effective_cache_size` | дефолт | 10 ГБ | планировщик |
| `work_mem` | дефолт | 32 МБ | сортировки/агрегаты |
| `maintenance_work_mem` | дефолт | 1 ГБ | vacuum/индексы |
| `checkpoint_timeout` | 5 мин | 15 мин | реже checkpoints |
| `max_wal_size` | 1 ГБ | 4–8 ГБ | реже checkpoints |
| `checkpoint_completion_target` | 0,5 | 0,9 | размазать запись |
| `bgwriter_lru_maxpages` | 100 | 1000 | сгладить сброс |
| `bgwriter_delay` | 200 мс | 50 мс | то же |
| `jit` | on (миграция) | off | OLTP не нужен |
| `autovacuum_naptime` | 1 мин | 30 с | быстрее уборка |
| `autovacuum_vacuum_cost_limit` | 200 | 2000 | быстрее vacuum |
| `random_page_cost` | 4 | 1,1 | SSD |
| `effective_io_concurrency` | 1 | 200 | SSD |
| `track_io_timing` | off | on | диагностика |

Дополнительно:

- [ ] Убрать `ALTER DATABASE dns_collector SET ...` из миграций; настройки — в `postgresql.conf`/`command` compose.
- [ ] Для `domain`/`ip`: `autovacuum_vacuum_scale_factor = 0,02`, `autovacuum_vacuum_threshold = 1000`.
- [ ] Проверить, что `shared_buffers` не приводит к OOM (VM 16 ГБ, Postgres в контейнере).
- [ ] Перезапуск с новыми настройками, замер через 24 ч.

**Критерий:** `buffers_checkpoint + buffers_clean + buffers_backend` ≤ 10 ГБ/сутки; checkpoint не чаще 1 раза в 15 мин.

### Этап 5. ZFS/VM (0,5–1 день + окно простоя)

- [ ] `zfs get volblocksize,compression,logbias,sync,atime <zvol>` — зафиксировать текущее.
- [ ] Пересоздать zvol с `volblocksize = 8K` (в PVE: новый zvol, копирование данных, переключение VM; либо backup/restore). `volblocksize` нельзя изменить на месте.
- [ ] `compression = zstd` (или `lz4`), `logbias = throughput`, `atime = off`.
- [ ] VM: `cache = none`, `aio = io_uring`, `discard = on`, `ssd = 1`; в гостевой ОС `noatime` и `fstrim.timer`.
- [ ] Если пересоздание невозможно в ближайшее время: включить компрессию и `logbias`, принять ×2 до следующего окна.

**Критерий:** `zpool iostat` показывает снижение физической записи; `smartctl` — темп износа.

### Этап 6. Планировщик резолвера (2 дня)

- [ ] Ввести `domain_schedule(domain_id, next_resolve_at, attempts, last_resolve_at)` с индексом только по `next_resolve_at`.
- [ ] Выборка: `WHERE next_resolve_at <= now() ORDER BY next_resolve_at LIMIT $1`.
- [ ] Backoff: `next_resolve_at = now() + min(base * 2^attempts, max)`; сброс `attempts` при изменении набора IP.
- [ ] Резолвить только домены, которые запрашивали за последние N дней (по `last_seen`); остальные — редко/никогда.
- [ ] `single-flight`: не запускать новый батч, если предыдущий ещё идёт.
- [ ] Отдельные контексты для IPv4 и IPv6; таймаут на каждый lookup.
- [ ] Батчевая запись результатов резолва в одной транзакции на домен.

**Критерий:** число резолвов/сутки снизилось в 2–4×; `UPDATE` по `domain` — только планировщик; нет наложения батчей.

### Этап 7. web-api и экспорт (2 дня)

- [ ] `limit`: верхняя граница (например, 1000) для обычных запросов; 413/400 при превышении.
- [ ] Экспорт Excel: `StreamWriter`, генерация по мере чтения из БД, без 100 тыс. строк в памяти.
- [ ] `GetDomainsWithIPs`: чанки по 1000 `domain_id` вместо `IN` на 100 тыс. (лимит параметров PostgreSQL — 65 535).
- [ ] Ранняя проверка `total > 100000` до выборки данных.
- [ ] `http.Server`: `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- [ ] `/health`: `Ping` БД.
- [ ] Валидация `client_ip` перед `::inet`; при невалидном — пустой результат, не 500.
- [ ] `LastSeen` — `*time.Time` или `sql.NullTime`.
- [ ] Ошибки — в лог, клиенту — общий текст.
- [ ] Убрать `defer f.Close()` из экспортёра до возврата файла.
- [ ] Date parse: 400 при невалидной дате.
- [ ] `ip_ttl_days = 0`: либо реально отключить, либо явно документировать «0 → 3».

**Критерий:** экспорт 100 тыс. строк без OOM; неизвестный `client_ip` не ломает фильтр; `limit=0` и `limit=10^9` не ломают пагинацию.

### Этап 8. Безопасность (4–5 дней, можно параллельно)

- [ ] Аутентификация на `web-api` (reverse proxy + OIDC/basic или middleware); закрыть публикацию 8080.
- [ ] UDP: allowlist source-IP на фаерволе + опциональный shared secret в пакете; валидация `domain`/`client_ip`.
- [ ] Убрать дефолтные пароли из репозитория, PostgreSQL не публиковать на 5432.
- [ ] `web-api` — non-root.
- [ ] Секреты — через env/secret store, не в `config.yaml`.
- [ ] Ограничить `/metrics` и export-эндпоинты по сети.
- [ ] Санитизация ошибок, `limit`, таймауты.

**Критерий:** без креденшелов API недоступен; UDP-пакеты с левого адреса отбрасываются; контейнеры non-root.

### Этап 9. Тесты, CI, наблюдаемость (3–4 дня)

- [ ] Интерфейсы БД в collector (`*database.Database` → `DB`), реальные тесты `handleMessage`, `runResolution`, `ApplyResolution`.
- [ ] `sqlmock`/Testcontainers для SQL; убрать мёртвые моки.
- [ ] Покрытие: `server`/`resolver`/`database` ≥60%.
- [ ] CI: одна версия Go (1.24), убрать `continue-on-error`, зафиксировать версию линтера, убрать `libsqlite3`.
- [ ] Frontend: lint + тесты (Vitest), `npm run test` в Makefile.
- [ ] Метрики: `db_writes_total`, `db_batch_size`, `db_queue_depth`, `db_errors_total`, `pg_stat_bgwriter` через postgres_exporter.
- [ ] Алерты: рост `n_tup_upd`, `buffers_backend`, размер `domain_stat`, dead tuples, autovacuum lag.
- [ ] Runbook: что делать при росте записи, как проверить HOT, как сделать бэкап/restore.

**Критерий:** CI краснеет при падении тестов; есть дашборд и алерты по записи; покрытие критичных пакетов ≥60%.

---

## 6. Детальные изменения

### 6.1. Приём UDP: батчинг и кэш доменов

Сейчас (упрощённо):

~~~go
// на каждый пакет
db.InsertDomainStat(domain, clientIP, rtype)
d, isNew, _ := db.InsertOrGetDomain(domain, maxResolv)
db.UpdateDomainLastSeen(d.ID)
log.Printf("Received DNS query: ...")
~~~

Предлагается:

~~~go
type Writer struct {
    db        *pgxpool.Pool
    domainIDs *lru.Cache[string]int64 // TTL 24h, max 100k
    stats     chan DomainStat          // 10k
    lastSeen  map[int64]time.Time      // flush every 60s
}

func (w *Writer) Handle(q DNSQuery) {
    id, ok := w.domainIDs.Get(q.Domain)
    if !ok {
        id = w.upsertDomain(q.Domain) // один раз на домен
        w.domainIDs.Add(q.Domain, id)
    }
    w.stats <- DomainStat{q.Domain, q.ClientIP, q.RType, time.Now()}
    w.touchLastSeen(id)
}
~~~

- Писатель статистики: батч 500 строк / 1 с, `pgx.CopyFrom`.
- `last_seen`: раз в 60 с один `UPDATE ... FROM unnest(...)`; условие «не чаще раза в 5 минут на домен».
- Канал ограничен; при переполнении — счётчик `db_dropped_total` и/или backpressure.
- `log.Printf` — только debug/ошибки.

### 6.2. `last_seen` без записи на каждый пакет

~~~sql
UPDATE domain d
SET last_seen = v.ts
FROM (
    SELECT * FROM unnest($1::bigint[], $2::timestamptz[]) AS v(id, ts)
) v
WHERE d.id = v.id
  AND (d.last_seen IS NULL OR d.last_seen < v.ts - interval '5 minutes');
~~~

На 15 тыс. доменов это один запрос раз в минуту вместо десятков тысяч отдельных `UPDATE`. Индекс по `last_seen` при этом не нужен: seq scan 15 тыс. строк — доли миллисекунды.

### 6.3. IP: не переписывать неизменное

Быстрый вариант без изменения схемы:

~~~sql
INSERT INTO ip (domain_id, ip, type, time)
VALUES ($1, $2, $3, $4)
ON CONFLICT (domain_id, ip) DO UPDATE
SET time = EXCLUDED.time
WHERE ip.time < EXCLUDED.time - interval '1 day';
~~~

Плюс в приложении:

1. Прочитать текущие IP домена (один `SELECT`, кэш на время батча).
2. Вычислить `toInsert`, `toKeep`, `toDelete`.
3. Вставить только новые; обновить `time` только для `toKeep` и не чаще раза в сутки; удалить `toDelete`.

Целевая схема (этап 6):

~~~sql
ALTER TABLE ip ADD COLUMN first_seen TIMESTAMP;
ALTER TABLE ip ADD COLUMN last_seen TIMESTAMP;
UPDATE ip SET first_seen = time, last_seen = time;
~~~

- `first_seen` — при вставке, никогда не обновляется.
- `last_seen` — обновляется раз в сутки (или при «возврате» IP).
- `time` оставить как `last_seen` для совместимости или переименовать в новой миграции.

### 6.4. Разделение `domain` на identity / activity / schedule

Целевая схема (после этапа 6):

~~~sql
CREATE TABLE domain (
    id          BIGSERIAL PRIMARY KEY,
    domain      TEXT NOT NULL UNIQUE,
    time_insert TIMESTAMP NOT NULL,
    max_resolv  INTEGER NOT NULL
);

CREATE TABLE domain_activity (
    domain_id BIGINT PRIMARY KEY REFERENCES domain(id) ON DELETE CASCADE,
    last_seen TIMESTAMP NOT NULL
) WITH (fillfactor = 70);

CREATE TABLE domain_schedule (
    domain_id       BIGINT PRIMARY KEY REFERENCES domain(id) ON DELETE CASCADE,
    next_resolve_at TIMESTAMP NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_resolved_at TIMESTAMP
) WITH (fillfactor = 70);

CREATE INDEX idx_domain_schedule_next ON domain_schedule(next_resolve_at);
~~~

- `domain` — почти immutable, обновляется только `max_resolv` (и то редко).
- `domain_activity` — узкая горячая таблица; `last_seen` без индекса, seq scan по 15 тыс. строк.
- `domain_schedule` — один индекс по `next_resolve_at`; обновление одной строки на резолв.
- `domain_view` для web-api, чтобы не менять API:

~~~sql
CREATE VIEW domain_view AS
SELECT d.id, d.domain, d.time_insert, d.max_resolv,
       COALESCE(s.attempts, 0) AS resolv_count,
       s.last_resolved_at AS last_resolv_time,
       a.last_seen
FROM domain d
LEFT JOIN domain_schedule s ON s.domain_id = d.id
LEFT JOIN domain_activity a ON a.domain_id = d.id;
~~~

Если разделение кажется слишком большим для первого релиза — сначала этап 2 (удалить индексы + `fillfactor`), этого уже достаточно, чтобы `UPDATE` по `domain` стали HOT.

### 6.5. Партиционирование `domain_stat`

~~~sql
CREATE TABLE domain_stat (
    id        BIGINT GENERATED ALWAYS AS IDENTITY,
    domain    TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype     TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

CREATE TABLE domain_stat_2026_11 PARTITION OF domain_stat
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
~~~

- Партиции по месяцам (30 дней retention → 2 живые партиции + текущая).
- Индексы на партицию: `(timestamp)`, `(client_ip, timestamp)`, `(domain, timestamp)`.
- Retention:

~~~sql
DROP TABLE IF EXISTS domain_stat_2026_08;
~~~

- `DELETE` оставить как fallback для нестандартных конфигураций.
- Перенос: создать новую таблицу, скопировать данные по месяцам, переименовать; старую оставить на релиз.

### 6.6. Индексный бюджет

По фактическому baseline 2026-11-23 (см. [DB_BASELINE.md](DB_BASELINE.md#6-инвентарь-индексов-и-вердикт)) суммарно можно убрать или заменить до **5,8 GB** индексов; безусловные дубли и индексы с нулевым `idx_scan` — ~3,3 GB.

| Таблица | Оставить | Удалить / заменить |
|---|---|---|
| `domain` | PK, `UNIQUE(domain)` | `idx_domain_name` (дубль, 224 MB), `idx_domain_time_insert` (0 scans, 53 MB), `idx_domain_resolv`/`idx_domain_resolv_count`/`idx_domain_resolv_lookup` (0 scans, 137 MB), `idx_domain_last_seen` (264 MB, 1 062 scans), `idx_domain_last_resolv_time` (34 MB, 47 933 scans — после переписывания запроса) |
| `ip` | `UNIQUE(domain_id, ip)`, `(ip)`, один `(time/last_seen)`* | `idx_ip_time` (дубль, 483 MB), `idx_ip_domain` (25 MB), `idx_ip_domain_id` (25 MB), `idx_ip_type` (20 MB), `idx_ip_domain_type` (20 MB); `ip_pkey` (182 MB) — опционально |
| `domain_stat` | `(timestamp)`, `(client_ip, timestamp)`, `(domain, timestamp)` | `domain_stat_pkey` (404 MB, 0 scans — опционально), `idx_domain_stat_time_client` (1 301 MB), `idx_domain_stat_client_ip` (208 MB), `idx_domain_stat_domain` (206 MB), `idx_domain_stat_rtype` (191 MB); `idx_domain_stat_time_domain` (2 064 MB) — заменить на `(domain, timestamp)` |

*`(time/last_seen)` нужен для очистки; после перехода на `last_seen` с суточным обновлением его стоимость невелика. Альтернатива — BRIN по `timestamp` для партиций и очистка по партициям. Для `domain_stat` PK не нужен: `id` не используется в запросах (0 scans), а identity можно оставить без уникального индекса.

### 6.7. PostgreSQL: конфигурация

Не менять через миграции. Пример для compose:

~~~yaml
postgres:
  image: postgres:16-alpine
  command:
    - postgres
    - -c
    - shared_buffers=4GB
    - -c
    - effective_cache_size=10GB
    - -c
    - work_mem=32MB
    - -c
    - maintenance_work_mem=1GB
    - -c
    - checkpoint_timeout=15min
    - -c
    - max_wal_size=8GB
    - -c
    - min_wal_size=2GB
    - -c
    - checkpoint_completion_target=0.9
    - -c
    - bgwriter_lru_maxpages=1000
    - -c
    - bgwriter_delay=50ms
    - -c
    - autovacuum_naptime=30s
    - -c
    - autovacuum_vacuum_cost_limit=2000
    - -c
    - jit=off
    - -c
    - random_page_cost=1.1
    - -c
    - effective_io_concurrency=200
    - -c
    - track_io_timing=on
    - -c
    - shared_preload_libraries=pg_stat_statements
~~~

- `shared_buffers` = 25% RAM, но не больше 8 ГБ для 16 ГБ VM.
- `max_wal_size` увеличивать осторожно: WAL сейчас ~21 ГБ/сутки, при батчах может вырасти.
- `jit=off` — убрать из миграции 000003.
- `statement_timeout` оставить, но не 30 с для cleanup: либо отдельная роль для cleanup, либо `SET LOCAL statement_timeout`.

### 6.8. ZFS/VM

Проверка:

~~~bash
zfs get -H volblocksize,compression,logbias,sync,atime nvme-mirror-pool/<zvol>
zpool iostat -v 1
smartctl -a /dev/nvme2
~~~

Пересоздание zvol:

1. Остановить VM, сделать бэкап/снапшот.
2. Создать новый zvol: `zfs create -V 100G -b 8K -o compression=zstd -o logbias=throughput -o atime=off nvme-mirror-pool/vm-201-disk-1-new`.
3. Подключить как второй диск, загрузиться с rescue, скопировать `dd if=/dev/... of=/dev/... bs=1M status=progress` (или backup/restore).
4. Переключить VM на новый диск, старый — в архив на неделю.
5. Проверить `zpool iostat` и `smartctl`.

VM: `cache=none`, `aio=io_uring`, `discard=on`, `ssd=1`; гость — `noatime`, `fstrim.timer`.

### 6.9. Резолвер

- `single-flight`: `atomic.Bool` или мьютекс; тик пропускается, если батч идёт.
- Отдельные контексты:
~~~go
ctx4, cancel4 := context.WithTimeout(ctx, timeout)
ips4, err4 := r.dnsConf.LookupIP(ctx4, "ip4", domain)
cancel4()
ctx6, cancel6 := context.WithTimeout(ctx, timeout)
ips6, err6 := r.dnsConf.LookupIP(ctx6, "ip6", domain)
cancel6()
~~~
- Настраиваемый upstream DNS (сейчас — системный `/etc/resolv.conf`).
- Батч результатов: `ApplyResolution` — одна транзакция на домен.
- Метрики: `resolver_batch_duration`, `resolver_skipped` (single-flight), `resolver_backoff`.

### 6.10. web-api

- `GetStats`: `COUNT(*)` можно оставить, но добавить `LIMIT`-валидацию и, при необходимости, кэш общего количества.
- `GetDomainsWithIPs`: чанки по 1000.
- Excel: `excelize.StreamWriter`.
- `http.Server` таймауты.
- `health` с `PingContext`.
- `LastSeen` nullable.
- `client_ip`: `net.ParseIP` до `::inet`; иначе не добавлять условие.
- `limit` ≤ 1000, `offset` ≥ 0.
- `metrics`: `db_*` метрики, `export_duration_seconds`.

---

## 7. Миграция и откат

### 7.1. Порядок выката

1. Этап 0: бэкап, baseline, флаги выключены.
2. Этап 1: приложение с батчингом/кэшем, флаги включены на 10% (или на dev), 24–72 ч наблюдения.
3. Этап 2: индексы и `fillfactor` — отдельная миграция, обратимая (индексы можно вернуть).
4. Этап 3: партиционирование — окно обслуживания; старая таблица сохраняется.
5. Этап 4: PostgreSQL — рестарт с новыми параметрами, окно 5–10 мин.
6. Этап 5: ZFS — окно 1–2 ч, только после подтверждения этапов 1–4.
7. Этапы 6–9 — по мере готовности.

### 7.2. Совместимость

- Новые таблицы/колонки — аддитивные; старый код продолжает работать.
- `domain_view` сохраняет имена колонок для web-api.
- `ip.time` не удалять до следующего релиза; писать и `time`, и `last_seen`.
- Feature flags позволяют вернуть старый путь без отката схемы.

### 7.3. Откат

- Приложение: предыдущий образ + флаги `false`.
- Индексы: пересоздать (на 15 тыс. строк — секунды).
- Партиции: переключить view/имя обратно на старую таблицу.
- PostgreSQL: вернуть старый `postgresql.conf` и рестартовать.
- ZFS: вернуть старый zvol (оставлен на неделю).
- Данные: `pg_restore` из дампа этапа 0.

### 7.4. Проверка данных

- `COUNT(*)` по `domain` и `ip` до/после.
- Сверка 100 случайных доменов: набор IP.
- Сверка `domain_stat` за последние сутки по количеству и примеру.
- Экспорт одного и того же regex до/после — diff.

---

## 8. Проверка результата

### 8.1. SQL для замера

~~~sql
-- Обновления и HOT
SELECT relname, n_tup_ins, n_tup_upd, n_tup_hot_upd, n_tup_del,
       n_live_tup, n_dead_tup, autovacuum_count, last_autovacuum
FROM pg_stat_user_tables
ORDER BY n_tup_upd DESC;

-- Буферы и checkpoints
SELECT * FROM pg_stat_bgwriter;

-- Размеры
SELECT relname,
       pg_size_pretty(pg_total_relation_size(relid)) AS total,
       pg_size_pretty(pg_relation_size(relid)) AS heap,
       pg_size_pretty(pg_total_relation_size(relid) - pg_relation_size(relid)) AS indexes
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC;

-- Индексы
SELECT relname, indexrelname, idx_scan,
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
ORDER BY pg_relation_size(indexrelid) DESC;
~~~

### 8.2. Внешние замеры

~~~bash
zpool iostat -v 1
iostat -x 1
smartctl -A /dev/nvme2
docker stats --no-stream dns-collector-postgres
~~~

### 8.3. Что считать успехом

- 7-дневное среднее `buffers_checkpoint + buffers_clean + buffers_backend` ≤ 10 ГБ/сутки.
- `n_tup_upd`: `ip` ≤ 0,1 млн/сутки, `domain` ≤ 0,2 млн/сутки.
- `n_tup_hot_upd / n_tup_upd` ≥ 0,9.
- `domain_stat`: 0 DELETE, размер ≤ 1,5 ГБ.
- `autovacuum_count` по `domain` ≤ 50/сутки.
- `zpool iostat` ≤ 0,6 МБ/с в среднем.
- p95 API ≤ 500 мс; экспорт 100 тыс. строк без OOM.
- Отсутствие потерь данных (сверка).

---

## 9. Риски

| Риск | Вероятность | Влияние | Митигация |
|---|---|---|---|
| Потеря статистики при батчинге | средняя | среднее | flush по таймеру, очередь, метрика потерь, `synchronous_commit=on` |
| Устаревший кэш `domain.id` | средняя | низкое | TTL, инвалидация при удалении, fallback на БД |
| Удаление нужного индекса | низкая | среднее | `EXPLAIN` до/после, мониторинг p95, быстрый откат |
| Блокировки при партиционировании | средняя | высокое | окно, копирование батчами, `ACCESS EXCLUSIVE` на секунды, старая таблица |
| `shared_buffers` слишком большой | низкая | высокое (OOM) | 25% RAM, мониторинг, постепенный рост |
| `fillfactor` не даёт HOT | средняя | среднее | проверить, что все индексы с обновляемых столбцов удалены |
| ZFS-пересоздание | средняя | высокое | бэкап, старый zvol, окно, проверка загрузки |
| Рост WAL при батчах | низкая | низкое | `max_wal_size`, мониторинг |
| Регресс API при переходе на `domain_view` | средняя | среднее | контрактные тесты, сравнение ответов |

---

## 10. Бэклог

| ID | Задача | Приоритет | Оценка | Зависит от | Эффект |
|---|---|---|---:|---|---|
| DB-0 | Бэкап, `pg_stat_statements`, baseline | P0 | 0,5 д | — | измерения |
| DB-1 | Кэш доменов, батчинг `domain_stat`, троттлинг `last_seen` | P0 | 2 д | DB-0 | −50…70% записей |
| DB-2 | IP-upsert только при изменении, удаление выпавших | P0 | 1 д | DB-1 | −80…95% `ip` UPDATE |
| DB-3 | Удаление индексов, `fillfactor`, запрос резолвера | P0 | 2 д | DB-2 | HOT ≥90% |
| DB-4 | Партиционирование `domain_stat`, DROP PARTITION | P1 | 3 д | DB-3 | −0,5 млн DELETE/сутки |
| DB-5 | Тюнинг PostgreSQL | P1 | 1 д | DB-4 | 1,2–2× |
| DB-6 | ZFS `volblocksize = 8K` | P1 | 1 д + окно | DB-5 | ×2 физической записи |
| DB-7 | `domain_schedule`/`domain_activity`, backoff, single-flight | P1 | 2–3 д | DB-3 | −2…4× резолвов |
| API-1 | Стриминг Excel, чанки `IN`, лимиты, таймауты, healthcheck | P1 | 2 д | — | стабильность экспорта |
| API-2 | Исправление `ip_ttl`, `inet`, `LastSeen`, дат, `limit=0` | P1 | 1 д | — | корректность |
| SEC-1 | Аутентификация web-api | P0 | 2 д | — | закрыть данные |
| SEC-2 | UDP allowlist/secret, валидация | P0 | 1 д | — | защита от spoofing |
| SEC-3 | Секреты, non-root, сеть, ошибки | P1 | 1 д | SEC-1 | гигиена |
| QUA-1 | Интерфейсы и тесты collector | P1 | 2 д | — | покрытие |
| QUA-2 | sqlmock/Testcontainers для SQL | P1 | 2 д | QUA-1 | покрытие |
| QUA-3 | CI: Go 1.24, убрать `continue-on-error`, линтер | P1 | 0,5 д | — | надёжность CI |
| QUA-4 | Frontend lint/tests, `npm run test` | P2 | 1 д | — | качество UI |
| OBS-1 | Метрики/алерты по записи и HOT | P1 | 1 д | DB-1 | контроль |
| DOC-1 | Актуализация документации | P0 | 1 д | — | достоверность |

---

## 11. Что уже сделано по документации

В рамках этой задачи:

- создан этот план;
- этап 1 (кэш доменов, батчинг `domain_stat`, троттлинг `last_seen`, upsert IP без перезаписи неизменных адресов) реализован в PR #17;
- снят и зафиксирован baseline боевого стенда 2026-11-23: [DB_BASELINE.md](DB_BASELINE.md) + сырые данные в [baseline/2026-11-23](baseline/2026-11-23/);
- включены `pg_stat_statements` и `track_io_timing`, снят первый query-level срез: [DB_STATISTICS.md](DB_STATISTICS.md);
- переписаны `ARCHITECTURE.md`, `QUICKSTART.md`, `INSTALL.md`, `DOCKER.md`, `SUMMARY.md`, `SERVICES.md`, `TESTING.md`;
- обновлены `README.md`, `QUICKSTART_WEBAPI.md`, `web-api/README.md`, `web-api/INSTALL.md`, `web-api/FEATURES.md`;
- создан `dns-collector/README.md` (битая ссылка из корневого README);
- актуализированы `PROJECT_STRUCTURE.txt`, `queries.sql`, `monitor.sh`;
- исправлены битые ссылки, убраны/помечены устаревшие снимки тестов;
- добавлена запись в `CHANGELOG.md`.

Документация описывает **текущее** состояние кода; пункты плана, которые ещё не реализованы, помечены в тексте как «план» и не выдаются за факт.

---

## Приложение A. Текущая схема (факт)

Таблицы создаются `initSchema()` и миграциями; на свежей БД фактически действует схема из `initSchema`, миграции лишь фиксируют версию.

~~~sql
domain (
    id SERIAL PRIMARY KEY,
    domain TEXT NOT NULL UNIQUE,
    time_insert TIMESTAMP NOT NULL,
    resolv_count INTEGER NOT NULL DEFAULT 0,
    max_resolv INTEGER NOT NULL,
    last_resolv_time TIMESTAMP NOT NULL,
    last_seen TIMESTAMP
);

ip (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER NOT NULL REFERENCES domain(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    type TEXT NOT NULL,
    time TIMESTAMP NOT NULL,
    UNIQUE(domain_id, ip)
);

domain_stat (
    id SERIAL PRIMARY KEY,
    domain TEXT NOT NULL,
    client_ip TEXT NOT NULL,
    rtype TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);
~~~

Индексы: `idx_domain_resolv`, `idx_domain_last_seen`, `idx_domain_name`, `idx_domain_resolv_count`, `idx_domain_last_resolv_time`, `idx_domain_time_insert`, `idx_domain_resolv_lookup`, `idx_ip_domain_id`, `idx_ip_type`, `idx_ip_time`, `idx_ip_address`, `idx_ip_domain_type`, `idx_domain_stat_timestamp`, `idx_domain_stat_client_ip`, `idx_domain_stat_domain`, `idx_domain_stat_rtype`, `idx_domain_stat_time_client`, `idx_domain_stat_time_domain`.

## Приложение B. Быстрые команды для проверки

~~~bash
# Размер БД
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT pg_size_pretty(pg_database_size('dns_collector'));"

# Топ таблиц
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT relname, pg_size_pretty(pg_total_relation_size(relid)) AS total
FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC;"

# HOT
docker compose exec -T postgres psql -U dns_collector -d dns_collector -c "
SELECT relname, n_tup_upd, n_tup_hot_upd,
       round(100.0 * n_tup_hot_upd / NULLIF(n_tup_upd,0), 1) AS hot_pct
FROM pg_stat_user_tables WHERE n_tup_upd > 0 ORDER BY n_tup_upd DESC;"

# Запись в пул
zpool iostat -v 1
~~~

## Приложение C. Ссылки на код

- `dns-collector/internal/server/udp.go` — приём и запись на каждый пакет.
- `dns-collector/internal/resolver/resolver.go` — резолв и upsert IP.
- `dns-collector/internal/database/database.go` — SQL, upsert, cleanup.
- `dns-collector/internal/cleanup/cleanup.go` — retention.
- `dns-collector/internal/config/config.go` — конфиг и defaults.
- `web-api/internal/database/database.go` — запросы UI/экспорта.
- `web-api/internal/handlers/handlers.go` — лимиты, экспорт, health.
- `dns-collector/internal/database/migrations/000003_performance_tuning.up.sql` — `ALTER DATABASE`.
