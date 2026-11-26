# 24-часовой профиль PostgreSQL после 2.9.1

**Окно:** 2026-11-25 08:49:01Z — 2026-11-26 08:47:39Z (23,98 ч)  
**Релиз:** dns-collector 2.9.1 / web-api 2.9.1  
**Срезов:** 286, ошибок сбора: 0  
**Статистика сброшена:** 2026-11-25 08:49:00Z

Профиль снят тем же [pg_monitor_24h.sh](../scripts/pg_monitor_24h.sh), что 2.9.0 и baseline. Host-часть (SSD, VM 1012, ZFS) — вне скоупа dns-collector; здесь только гостевая запись и PostgreSQL.

## 1. Итог

Этап 2 сработал сильнее, чем ожидалось: WAL упал ещё в 2 раза, запись гостевого диска — ещё в 2 раза, размер БД уменьшился на 3,57 GB.

| Метрика | Baseline 2.8.0 | 2.9.0 | **2.9.1** | 2.9.1 vs 2.9.0 |
|---|---:|---:|---:|---:|
| WAL | 20,83 GB/сутки | 7,52 | **3,69** | **−51%** |
| Запись `sdb1` (VM) | 89,08 GB/сутки | 36,60 | **18,90** | **−48%** |
| Буферы PostgreSQL | 50,2 GB/сутки | 21,18 | **8,04** | **−62%** |
| `checkpoint_write_time` | 90,1% | 90,0% | **83,7%** | −6,3 п.п. |
| `domain` UPDATE | 1,13 млн/сутки | 831 тыс. | **829 тыс.** | −0,2% |
| `domain` HOT | 0,02% | 0% | **13,3%** | +13,3 п.п. |
| `ip` UPDATE строк | 1,72 млн/сутки | 108 тыс. | **141 тыс.** | +30,6% |
| `ip` HOT | 0% | 0% | **99,7%** | +99,7 п.п. |
| `domain_stat` INSERT, строк | 412,6 тыс. | 454,2 тыс. | **438,3 тыс.** | −3,5% |
| `domain_stat` DELETE | 368,9 тыс. | 415,1 тыс. | **400,5 тыс.** | −3,5% |
| `domain_stat` dead tuples | 439 884 | 499 380 | **365 832** | −26,7% |
| `domain` autovacuum | 3 385 | 3 750 | **3 996** | +6,6% |
| Размер БД | 7 755 MB | 7 759 MB | **4 191 MB** | **−3 568 MB** |
| Размер индексов | 7 237 MB | 7 237 MB | **~3 660 MB** | **−3 577 MB** |

Источник ускорения — удаление 13 индексов и `fillfactor=70`: `ip` стал почти полностью HOT, `domain_stat` перестал платить за четыре индекса на INSERT, уменьшились checkpoint и autovacuum.

## 2. Индексы и размер БД

Удалено ~**3,57 GB** индексов. БД: **4 191 MB**, из них индексы **~3 660 MB (87,3%)**, heap ~530 MB.

| Таблица | Total | Heap | Indexes | Live | Dead |
|---|---:|---:|---:|---:|---:|
| `domain_stat` | 3 196 MB | 305 MB | **2 891 MB** | 3 186 091 | 365 832 |
| `domain` | 514 MB | 115 MB | 398 MB | 26 153 | 1 167 |
| `ip` | 473 MB | 102 MB | 371 MB | 442 336 | 63 762 |

`n_live_tup` — оценка статистики; по метрикам на конец окна: 16 302 домена, 443 033 IP.

Осталось:

- `domain_stat`: `idx_domain_stat_time_domain` 2 064 MB (52 scan), `idx_domain_stat_timestamp` 422 MB (366), `domain_stat_pkey` 404 MB (**0 scan**);
- `domain`: `domain_domain_key` 311 MB (5,8 млн scan), `domain_pkey` 54 MB, `idx_domain_last_resolv_time` 34 MB (63,5 тыс.);
- `ip`: `ip_pkey` 182 MB (**0 scan**), `ip_domain_id_ip_key` 95 MB (29,7 млн), `idx_ip_address` 30 MB, `idx_ip_type` 20 MB, `idx_ip_domain_type` 20 MB, `idx_ip_domain_id` 25 MB.

Следующий крупный шаг — партиционирование `domain_stat`: это уберёт pkey и `time_domain`, а retention станет `DROP PARTITION`.

## 3. Куда ушёл WAL

`pg_stat_statements` учёл **3,19 GB** из 3,69 GB системного WAL (86,6%).

| Запрос | Вызовов | WAL, GB | Доля |
|---|---:|---:|---:|
| `UPDATE ip` (refresh) | 870 502 | **0,826** | **22,4%** |
| `UPDATE domain` (resolv_count) | 717 000 | **0,752** | **20,4%** |
| `INSERT ip` (DO NOTHING) | 870 502 | **0,477** | **12,9%** |
| `DELETE domain_stat` | 22 | **0,432** | **11,7%** |
| `INSERT domain_stat` | 51 479 | **0,221** | **6,0%** |
| `SELECT COUNT(*) FROM domain` | 2 867 | 0,135 | 3,7% |
| `SELECT COUNT(*) FROM ip` | 2 867 | 0,126 | 3,4% |
| `DELETE ip` (retention) | 22 | 0,115 | 3,1% |
| `DELETE ip` (старые домены) | 22 | 0,081 | 2,2% |
| `UPDATE domain last_seen` | 1 434 | 0,027 | 0,7% |
| Прочее | — | ~0,49 | ~13% |

Что изменилось против 2.9.0:

- `UPDATE ip`: 2,378 → **0,826 GB** (−65%), при этом HOT вырос до 99,7%;
- `UPDATE domain`: 1,429 → **0,752 GB** (−47%);
- `INSERT domain_stat`: 1,276 → **0,221 GB** (−83%);
- `last_seen`: 0,587 → **0,027 GB** (−95%);
- `DELETE domain_stat`: 0,487 → **0,432 GB** (−11%).

Оставшиеся крупные статьи — не «лишние индексы», а количество вызовов и архитектура:

1. `UPDATE ip` — 870 тыс. вызовов, по одному на каждый успешный резолв IPv4/IPv6. Даже при 99,7% HOT это 0,826 GB и заметный CPU.
2. `UPDATE domain` — 717 тыс. вызовов счётчика; HOT только 13,3%, потому что `idx_domain_last_resolv_time` и существующие полные страницы.
3. `DELETE domain_stat` — 400 тыс. строк/сутки; нужен `DROP PARTITION`.
4. `SELECT COUNT(*)` каждые 30 секунд — 0,26 GB WAL/сутки; можно отдать `reltuples`/кэшу или снизить частоту.

## 4. Диск, checkpoint, HOT

| Метрика | 2.9.0 | 2.9.1 |
|---|---:|---:|
| Запись `sdb1` | 36,60 GB/сутки | **18,90 GB/сутки** |
| Чтение `sdb1` | 15,92 GB/сутки | **4,35 GB/сутки** |
| `sdb1` write | 0,424 MB/s | **0,219 MB/s** |
| Буферы checkpoint / bgwriter / backend | 1,19M / 1,06M / 0,33M | **0,73M / 0,20M / 0,06M** |
| WAL FPI | 2,45M | **0,99M** |
| WAL records | 17,6M | **8,58M** |
| Cache hit | 93,6% | **95,0%** |
| Checkpoint’ов timed / req | 288 / 0 | **288 / 0** |

`checkpoint_write_time` = 72 245 100 мс = **83,7%** календарного времени (было 90,0%). Checkpoint всё ещё почти непрерывный, но грязных страниц стало в 2 раза меньше. Это уже не «checkpointer пишет WAL», а остаточная стоимость 288 checkpoint’ов и ZFS; ZFS — вне скоупа.

HOT:

- `ip`: **99,71%** — удаление индексов по `time`/type сработало;
- `domain`: **13,27%** — `idx_domain_last_resolv_time` и полные существующие страницы не дают fillfactor проявиться полностью.

## 5. Autovacuum

| Таблица | autovacuum | Dead | Live |
|---|---:|---:|---:|
| `domain` | **3 996** | 1 167 | 26 153 |
| `ip` | 253 | 63 762 | 442 336 |
| `domain_stat` | 11 | **365 832** | 3 186 091 |

Autovacuum `domain` остался высоким: 3 996 проходов/сутки, потому что обновления счётчика всё ещё не HOT. `domain_stat` dead tuples упали на 26,7%, но 365 тыс. — всё ещё много; партиционирование нужно.

## 6. Что делать дальше

### 6.1. Применить fillfactor к существующим страницам

`domain` — маленькая таблица (115 MB heap, 26 тыс. строк). `VACUUM FULL domain` или `pg_repack` в короткое окно должен поднять HOT с 13% и снизить autovacuum. Это самый дешёвый следующий шаг.

Для `ip` это не нужно: HOT уже 99,7%.

### 6.2. Партиционировать `domain_stat`

- партиции по дню/месяцу, retention через `DROP PARTITION`;
- убрать `domain_stat_pkey` и `idx_domain_stat_time_domain` на партициях;
- индексы: `(timestamp)` и `(domain, timestamp)`;
- ожидаемо: −0,43 GB WAL на DELETE, −365 тыс. dead tuples, −2 GB индексов.

### 6.3. Код: `UpsertIPs`

- не выполнять `UPDATE ip`, если ни один адрес не старше `ip_refresh_hours`;
- объединить IPv4+IPv6 в одну транзакцию на домен (сейчас 870 тыс. BEGIN/COMMIT);
- это должно убрать большую часть 0,826 GB WAL и 0,477 GB INSERT-WAL, даже при HOT.

### 6.4. `SELECT COUNT(*)`

DB-коллектор каждые 30 секунд делает два `COUNT(*)` по 3,2 млн и 442 тыс. строк, это 0,26 GB WAL/сутки. Варианты: `reltuples`, кэш на 5–10 минут или отказ от точного счётчика в метрике.

## 7. Критерии следующего замера

- WAL < 2,5 GB/сутки;
- `sdb1` < 12 GB/сутки;
- `checkpoint_write_time` < 70%;
- `domain` HOT > 50%;
- `domain` autovacuum < 2 000/сутки;
- `domain_stat` dead tuples < 100 тыс.;
- отсутствие роста ошибок/дропов.

## 8. Данные

Сырые данные: [docs/baseline/2026-11-26/2026-11-25](baseline/2026-11-26/2026-11-25/).

- `meta.txt`, `monitor.log`, `errors.log`;
- `pg_stat_statements.csv.gz`, `tables.csv`, `pg_stat_io.csv`;
- `system.csv`, `disk.csv`, `docker.csv`;
- `collector-metrics.log.gz`, `web-api-metrics.log.gz`;
- `analysis.json`.
