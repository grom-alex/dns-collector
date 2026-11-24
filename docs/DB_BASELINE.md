# Baseline: dns-collector / PostgreSQL на 192.168.0.15

**Дата снятия:** 2026-11-23, 00:14–00:22 +06 (2026-11-22 18:14–18:22 UTC); основной снимок — 00:14–00:18, 5-минутный срез — 00:17–00:22  
**Стенд:** `docker-debian`, VM на PVE, 192.168.0.15  
**Сервисы:** dns-collector 2.8.0, web-api 2.8.0, PostgreSQL 16.11  
**Режим:** только чтение (SSH, `psql SELECT`, `/proc`, `docker stats`, `curl`). Изменений в БД и сервисах не вносилось.

> Это опорный срез «до оптимизации». Все последующие изменения сравниваются с ним по одним и тем же запросам (раздел 12). Сырые данные — в [docs/baseline/2026-11-23](baseline/2026-11-23/).
>
> **Post-baseline change (2026-11-23 00:35 +06):** включены `pg_stat_statements` и `track_io_timing`. В этом baseline query-level статистики нет; первый срез — в [DB_STATISTICS.md](DB_STATISTICS.md), суточный профиль — в [DB_MONITORING_REPORT.md](DB_MONITORING_REPORT.md).

---

## 1. Паспорт стенда

| Параметр | Значение |
|---|---|
| Хост | `docker-debian` |
| ОС | Debian GNU/Linux 12 (bookworm) |
| Ядро | `6.1.0-33-amd64` |
| CPU / RAM | 8 vCPU / 15 GiB (использовано ~4,2 GiB, cache ~12 GiB) |
| Uptime VM | 10 дней 7:31 |
| PostgreSQL | 16.11, `pg_postmaster_start_time = 2026-11-12 10:42:45 UTC` |
| PG uptime | 10 дней 7:35, не в recovery |
| Размер БД | **7 755 MB** |
| WAL на диске | 192 MB |
| Данные PG | `/mnt/pgdata` (`/dev/sdb1`, ext4, 63 GB, занято 12 GB) |
| Корневой диск | `/dev/sda1`, ext4, 62 GB, занято 42 GB (**71%**) |
| Docker | compose-проект `/opt/dns-collector` |
| Collector | UDP 5353, метрики 8081, InfluxDB push включён |
| Web API | 8080, метрики на том же порту |
| Окно счётчиков | `pg_stat_bgwriter.stats_reset = 2026-11-11 15:40:56 UTC` → ~**11,1 суток** |

Сырые данные: [system-services.txt](baseline/2026-11-23/system-services.txt), [postgres-snapshot.txt](baseline/2026-11-23/postgres-snapshot.txt), [collector-metrics.txt](baseline/2026-11-23/collector-metrics.txt), [web-api-metrics.txt](baseline/2026-11-23/web-api-metrics.txt).

---

## 2. Методика и ограничения

- Снимок PostgreSQL — `pg_stat_*`, `pg_class`, `pg_indexes`, `pg_settings`; без `pg_stat_statements` (расширение не установлено).
- Ставки DML/буферов посчитаны как «значение / 11,108 суток» (окно `stats_reset`).
- Дополнительно снят 60-секундный срез [rates-60s.txt](baseline/2026-11-23/rates-60s.txt) и 5-минутный [rates-300s.txt](baseline/2026-11-23/rates-300s.txt).
- 60-секундный срез шумный: резолвер работает батчами по 1 000 доменов раз в 120 с, поэтому мгновенная ставка скачет. Для устойчивых чисел используйте накопленные значения и 5-минутный срез.
- `track_io_timing = off`, поэтому `read_time/write_time` в `pg_stat_io` и `wal_write_time` равны 0; тайминги I/O недоступны.
- ZFS-статистика хоста PVE из VM не видна: `zpool iostat`, `volblocksize` и `smartctl` нужно снимать на PVE. В отчёте указано `volblocksize = 16K`; из VM это не проверить.
- `pg_stat_statements` на момент снимка отсутствовал — топ запросов по `total_exec_time`/`wal_bytes` в этом baseline нет. Он включён после снятия baseline (2026-11-23 00:35), см. [DB_STATISTICS.md](DB_STATISTICS.md).

---

## 3. Главное

1. **Запись подтверждается.** PostgreSQL пишет ~**49,9 ГБ/сутки** буферами (`bgwriter + checkpointer + backend`) и ~**21,2 ГБ/сутки** WAL. Диск `sdb1` принял **89,1 ГБ/сутки** (`1,03 МБ/с` в среднем за 10,3 суток; 5-минутный срез — `0,85 МБ/с`). Это тот же порядок, что в отчёте про 2,6 МБ/с на пуле с учётом ×2 ZFS и других VM.
2. **93,3% БД — индексы.** Из 7 755 MB: heap ~509 MB, индексы **7 237 MB**. У `domain` 1 076 MB индексов на 16 279 строк (67,7 KB/строку), у `ip` 1 363 MB на 400 277 строк, у `domain_stat` 4 798 MB на 3,1 млн строк.
3. **DML-поток:** `domain` — ~1,14 млн UPDATE/сутки, HOT 0,021%; `ip` — ~1,71 млн UPDATE/сутки, HOT 0%; `domain_stat` — ~0,42 млн INSERT и ~0,41 млн DELETE/сутки.
4. **Checkpointer пишет почти непрерывно:** 3 193 плановых checkpoint за 11,1 суток (каждые ~5 мин), суммарное `checkpoint_write_time` = 861 273 с = **89,7% календарного времени**.
5. **Autovacuum — крупный писатель и главный читатель:** 10,9 млн буферов записи (**16%** записи PG) и 541 млн буферов чтения (**92,7%** всех `blks_read`). По `domain` — 3 107 проходов за 11,1 суток (~280/сутки, почти непрерывно).
6. **Конфигурация коллектора усиливает проблему:** `resolver.interval_seconds = 120`, `workers = 100`, батч **1 000 доменов** каждые 2 минуты; `stats_days = 7`, cleanup каждый час; `domain_ttl_days = 14`.
7. **Логи и IPv6:** ~122 MB/сутки и ~1,28 млн строк/сутки; 20 921 ошибка IPv6 в час (71% IPv6-lookup'ов) и 2 830 ошибок IPv4 в час.
8. **Риски эксплуатации:** корневой диск занят на 71%; `pg_stat_statements` и `track_io_timing` выключены; web-api без аутентификации; PostgreSQL опубликован на 5432.

---

## 4. DML: накопленные значения и ставки

Окно ~11,108 суток. `per day` — оценка по накопленному значению; `per sec` — для сравнения с потоком пакетов.

| Метрика | Накоплено | В сутки | В секунду |
|---|---:|---:|---:|
| `domain` INSERT | 8 485 | 764 | 0,009 |
| `domain` UPDATE | **12 708 734** | **1 144 034** | **13,2** |
| `domain` HOT UPDATE | 2 679 | 241 | 0,003 |
| `domain` DELETE | 8 400 | 756 | 0,009 |
| `ip` INSERT | 532 893 | 47 970 | 0,56 |
| `ip` UPDATE | **19 039 453** | **1 713 999** | **19,8** |
| `ip` HOT UPDATE | **0** | 0 | 0 |
| `ip` DELETE | 589 609 | 53 080 | 0,61 |
| `domain_stat` INSERT | 4 719 139 | 424 839 | 4,9 |
| `domain_stat` DELETE | 4 597 422 | 413 884 | 4,8 |

Доли HOT:

- `domain`: 2 679 / 12 708 734 = **0,021%**.
- `ip`: **0%**.

Проверка сходимости с потоком:

- UDP-сообщений: ~398 тыс./сутки (4,6/с) — см. раздел 9.
- Резолвов доменов: ~669 тыс./сутки.
- `domain` UPDATE ≈ 398 тыс. (`last_seen` на пакет) + 669 тыс. (`resolv_count/last_resolv_time` на резолв) + запас = 1,07–1,14 млн/сутки. Сходится.
- `ip` UPDATE / резолв ≈ 1,71 млн / 669 тыс. ≈ **2,56 IP на домен за резолв**.

---

## 5. Размеры таблиц и индексов

| Таблица | Всего | Heap | Индексы | Доля индексов | Живых строк |
|---|---:|---:|---:|---:|---:|
| `domain_stat` | 5 090 MB | 292 MB | **4 798 MB** | **94,3%** | 3 096 792 |
| `ip` | 1 466 MB | 102 MB | **1 363 MB** | **93,0%** | 400 277 |
| `domain` | 1 192 MB | 115 MB | **1 076 MB** | **90,3%** | 16 279 |
| `schema_migrations` | 56 kB | 8 kB | 48 kB | — | 0 |
| **Итого** | **7 755 MB** | **~509 MB** | **7 237 MB** | **93,3%** | — |

Байт на строку (грубая оценка, включает bloat):

| Таблица | Heap/строка | Индексы/строка |
|---|---:|---:|
| `domain` | 7,2 KB | **67,7 KB** |
| `ip` | 267 B | **3,49 KB** |
| `domain_stat` | 98 B | **1,59 KB** |

Вывод: это не «нормальные» индексы, а раздутые структуры. Для 16 тыс. доменов один btree-индекс должен занимать единицы MB, а не 1 GB. Причина — миллионы не-HOT UPDATE, из-за которых каждая версия строки оставляет записи во всех индексах; vacuum не возвращает место, а страницы не переиспользуются полностью.

---

## 6. Инвентарь индексов и вердикт

`idx_scan` — с момента `stats_reset` (11,1 суток).

### `domain`

| Индекс | Размер | `idx_scan` | Вердикт |
|---|---:|---:|---|
| `domain_domain_key` (unique domain) | 311 MB | 5 135 856 | оставить, пересоздать (bloat) |
| `idx_domain_last_seen` | 264 MB | 1 062 | удалить; заменить seq scan/activity-таблицей |
| `idx_domain_name` (дубль unique) | 224 MB | 4 710 887 | **удалить** (уникальный индекс покрывает) |
| `domain_pkey` | 54 MB | 14 172 642 | оставить |
| `idx_domain_time_insert` | 53 MB | 0 | **удалить** |
| `idx_domain_resolv_lookup` | 53 MB | 0 | **удалить** |
| `idx_domain_resolv_count` | 42 MB | 0 | **удалить** |
| `idx_domain_resolv` | 42 MB | 0 | **удалить** |
| `idx_domain_last_resolv_time` | 34 MB | 47 933 | удалить после переписывания запроса резолвера |

### `domain_stat`

| Индекс | Размер | `idx_scan` | Вердикт |
|---|---:|---:|---|
| `idx_domain_stat_time_domain` | **2 064 MB** | 36 | заменить на `(domain, timestamp)` после партиционирования |
| `idx_domain_stat_time_client` | **1 301 MB** | 0 | **удалить** |
| `idx_domain_stat_timestamp` | 422 MB | 269 | оставить |
| `domain_stat_pkey` | 404 MB | 0 | не нужен: `id` не ищется; можно не создавать в партициях |
| `idx_domain_stat_client_ip` | 208 MB | 0 | **удалить** |
| `idx_domain_stat_domain` | 206 MB | 0 | **удалить** |
| `idx_domain_stat_rtype` | 191 MB | 0 | **удалить** |

### `ip`

| Индекс | Размер | `idx_scan` | Вердикт |
|---|---:|---:|---|
| `idx_ip_time_cleanup` | 484 MB | 0 | оставить один; рассмотреть BRIN/партицию |
| `idx_ip_time` | 483 MB | 0 | **удалить** — дубль |
| `ip_pkey` | 182 MB | 0 | кандидат на удаление (id не используется) |
| `ip_domain_id_ip_key` (unique) | 95 MB | 19 572 353 | оставить |
| `idx_ip_address` | 30 MB | 17 | оставить для shared-IP экспорта |
| `idx_ip_domain` | 25 MB | 0 | **удалить** — дубль `idx_ip_domain_id` |
| `idx_ip_domain_id` | 25 MB | 23 103 | **удалить** — покрыт unique `(domain_id, ip)` |
| `idx_ip_type` | 20 MB | 52 722 | **удалить** — низкая селективность |
| `idx_ip_domain_type` | 20 MB | 44 335 | **удалить** — покрыт unique по `domain_id` |

**Потенциал:** до **5,8 GB** индексов можно удалить или заменить. Из них безусловные дубли/нулевые scans — ~3,3 GB; ещё ~2,1 GB — `idx_domain_stat_time_domain`, который надо заменить, а не просто удалить. После пересоздания индексов bloat уйдёт, а `domain_domain_key`/unique станут в разы меньше.

---

## 7. Запись PostgreSQL: кто и сколько

Источник — `pg_stat_io` за 11,108 суток. 1 буфер = 8 KB.

| Источник | Буферов | ГБ | ГБ/сутки | Доля записи |
|---|---:|---:|---:|---:|
| background writer | 33 426 890 | 273,8 | **24,6** | 49,4% |
| checkpointer | 20 705 065 | 169,6 | **15,3** | 30,6% |
| autovacuum worker | 10 922 748 | 89,5 | **8,1** | 16,1% |
| client backend | 2 535 797 | 20,8 | 1,9 | 3,7% |
| прочие | 128 138 | 1,0 | 0,1 | 0,2% |
| **Итого** | **67 718 638** | **554,8** | **49,9** | **100%** |

WAL:

| Метрика | Значение | В сутки |
|---|---:|---:|
| `wal_bytes` | 235 696 501 859 B (235,7 GB) | **21,2 GB** |
| `wal_records` | 481 895 524 | 43,4 млн |
| `wal_fpi` | 60 439 722 | 5,44 млн |
| `wal_buffers_full` | 267 746 | 24,1 тыс. |

Итого логической записи: **~71 ГБ/сутки** (буферы + WAL). Диск `sdb1` — **89 ГБ/сутки**; разница — журнал ext4, метаданные, служебные записи, неточность окон.

### Checkpoints

| Метрика | Значение | Комментарий |
|---|---:|---|
| `checkpoints_timed` | 3 193 | ~287/сутки, каждые 5,0 мин |
| `checkpoints_req` | 4 | запрошенные |
| `checkpoint_write_time` | 861 273 170 ms = 861 273 с | **89,7%** календарного времени |
| `checkpoint_sync_time` | 122 642 ms | 0,013%, sync не проблема |
| `buffers_checkpoint` | 20 705 065 | ~6 480 буферов/checkpoint = 53 MB |
| `maxwritten_clean` | 116 384 | bgwriter упирается в лимит 100 страниц |
| `buffers_backend` | 13 586 683 | backend сам сбрасывает страницы |

### Autovacuum

| Метрика | `domain` | `ip` | `domain_stat` |
|---|---:|---:|---:|
| `autovacuum_count` | 3 107 | 219 | 9 |
| В сутки | ~280 | ~20 | ~0,8 |
| `n_dead_tup` на снимке | 1 154 | 64 969 | 52 270 |

Autovacuum worker за 11,1 суток:

- записал 10 922 748 буферов = **89,5 GB** (16,1% записи);
- прочитал 541 447 655 буферов = **4,44 TB** (92,7% всех `blks_read`, ~399 GB/сутки);
- на `domain` — почти непрерывный цикл: 1,14 млн UPDATE/сутки при пороге `0,2 × 16 279 ≈ 3 256` мёртвых строк.

Это объясняет, почему «вакуум крутится постоянно»: он не успевает за не-HOT UPDATE, а раздутые индексы заставляют его читать сотни миллионов страниц.

### Кэш и чтение

| Метрика | Значение |
|---|---:|
| `blks_read` | 584 165 118 (4,79 TB) |
| `blks_hit` | 7 866 051 849 (64,4 TB) |
| Cache hit ratio | **93,1%** |
| `xact_commit` | 95 917 925 (~8,6 млн/сутки) |
| `xact_rollback` | 405 |
| `temp_files` / `temp_bytes` | 0 / 0 |
| `deadlocks` | 0 |

При `shared_buffers = 128 MB` и 15 GiB RAM кэш в основном в page cache ОС; 93% hit — не повод не увеличивать `shared_buffers`, потому что запись грязных страниц идёт постоянно.

---

## 8. Диск и ОС

### Накопленная запись

| Устройство | Запись (sectors × 512) | Всего | В сутки | В секунду |
|---|---:|---:|---:|---:|
| `sdb1` (pgdata) | 1 796 295 656 | **919,7 GB** | **89,1 GB** | **1,03 MB/s** |
| `sda1` (root) | 41 925 368 | 21,5 GB | 2,08 GB | 24 KB/s |
| `sdc` (nexus) | 7 124 944 | 3,65 GB | 0,35 GB | 4 KB/s |

Чтение `sdb1`: 6 915 431 866 sectors = **3,54 TB** за 10,32 суток = ~343 GB/сутки (~3,98 MB/s).

### 60-секундный срез

| Метрика | Значение |
|---|---:|
| Запись `sdb1` | 43,5 MB / 63 с = **691 KB/s** |
| Чтение `sdb1` | 3,37 MB / 63 с = 53 KB/s |
| `buffers_clean` | +2 863 |
| `buffers_checkpoint` | +824 |
| `buffers_backend` | +1 370 |
| `wal_bytes` | +5,87 MB (93 KB/s) |

Мгновенная запись ниже средней, потому что 60-секундное окно не попало на пик checkpoint/резолвера.

### 5-минутный срез

Окно 00:17:37–00:22:37 (300 с), [rates-300s.txt](baseline/2026-11-23/rates-300s.txt):

| Метрика | Δ за 300 с | Ставка | В сутки |
|---|---:|---:|---:|
| Запись `sdb1` | 255,4 MB (498 912 sectors) | **851 KB/s** | 73,5 GB |
| Чтение `sdb1` | 1,086 GB (2 120 768 sectors) | 3,62 MB/s | 313 GB |
| `buffers_checkpoint` | +7 861 | 215 KB/s | 18,5 GB |
| `buffers_clean` | +7 123 | 195 KB/s | 16,8 GB |
| `buffers_backend` | +1 914 | 52 KB/s | 4,5 GB |
| Итого буферов PG | +16 898 | **461 KB/s** | **39,9 GB** |
| `wal_bytes` | +63,3 MB | 211 KB/s | 18,2 GB |
| `domain` UPDATE | +3 362 | 11,2/s | 968 тыс. |
| `ip` UPDATE | +4 856 | 16,2/s | 1,40 млн |
| `domain_stat` INSERT | +1 363 | 4,5/s | 392 тыс. |
| `checkpoint_write_time` | +269 842 ms | 89,9% времени | — |
| Autovacuum writes | +1 492 | 41 KB/s | 3,5 GB |
| Autovacuum reads | +165 981 | 4,53 MB/s | 392 GB |

5-минутный срез ниже среднего за 11 суток (71 ГБ логической записи, 89 ГБ на диск): в окно не попал часовой cleanup, а активность резолвера/autovacuum колеблется. Для KPI надо смотреть и среднее за сутки, и 5-минутное окно.

### Docker I/O

| Контейнер | Block I/O | CPU | RAM |
|---|---:|---:|---:|
| postgres | **3,54 TB / 910 GB** | 0,26% | 453 MB / 1 GiB |
| dns-collector | 10,1 MB / 0 B | 0,26% | 16,9 MB / 512 MiB |
| web-api | 26,2 MB / 0 B | 0,28% | 23,4 MB / 512 MiB |

Postgres I/O почти полностью совпадает с `/proc/$PID/io`: `read_bytes = 3,537 TB`, `write_bytes = 490,8 GB`. `write_bytes` ниже `diskstats`, потому что учитывает write-системные вызовы, а не журнал/метаданные ФС.

### Файловая система

- `/mnt/pgdata`: ext4, `rw,relatime`; `fstrim.timer` включён и активен; диск не ротационный, scheduler `none`.
- `vm.dirty_bytes = 1 GiB`, `vm.dirty_background_bytes = 256 MiB`; на снимке `Dirty = 856 kB`, `Writeback = 0`.
- Корневой диск занят на 71% (42 из 62 GB); 18 GB свободно. При 122 MB/сутки логов и росте образов это требует контроля.

### ZFS

Из VM виден только virtio-диск `sdb`; `volblocksize`, `compression`, `zpool iostat` и `smartctl` нужно снимать на PVE. В отчёте указано `volblocksize = 16K` при странице 8K; это подтверждает необходимость этапа 5 плана, но в этом baseline хостовые ZFS-метрики отсутствуют.

---

## 9. Приложение: поток, резолвер, очистка, логи

### UDP и обработка

| Метрика | Накоплено (11,1 сут) | В сутки | В секунду |
|---|---:|---:|---:|
| `dns_server_messages_received_total{status="valid"}` | 4 425 400 | 398 400 | 4,61 |
| `...{status="invalid"}` | 750 | 68 | 0,001 |
| `dns_server_new_domains_total` | 7 848 | 706 | 0,008 |
| `dns_server_processing_duration_seconds_sum` | 237 158 с | — | — |
| Среднее время обработки | **53,6 ms** | — | — |

Распределение времени обработки: ≤50 ms — 67%, ≤100 ms — 86%, >100 ms — 14% (хвост до 10 s). При 4,6 пакета/с и неограниченных горутинах это создаёт заметный backlog при всплесках.

### Резолвер

| Метрика | Значение | В сутки |
|---|---:|---:|
| `dns_resolver_domains_processed_total{status="success"}` | 6 826 532 | 614 600 |
| `...{status="no_results"}` | 600 468 | 54 100 |
| Lookup IPv4 success / error | 6 793 716 / 633 284 | — |
| Lookup IPv6 success / error | 2 157 302 / 5 269 698 | — |
| `dns_resolver_lookup_duration_seconds_count{ip_version="ipv4"}` | 7 427 000 | 668 600 |
| `...{ip_version="ipv6"}` | 7 427 000 | 668 600 |

**71% IPv6-lookup'ов заканчиваются ошибкой** (нет AAAA), при этом IPv6-запрос выполняется для каждого домена. Это лишние DNS-запросы, логи и нагрузка. IPv4-ошибок 8,5%.

### Очистка

| Метрика | Накоплено | В сутки |
|---|---:|---:|
| `dns_cleanup_runs_total` | 248 | 22,3 (раз в час) |
| `dns_cleanup_stats_deleted_total` | 4 282 367 | 385 500 |
| `dns_cleanup_ips_deleted_total` | 245 438 | 22 100 |
| `dns_cleanup_domains_deleted_total` | 7 979 | 718 |
| `dns_cleanup_domain_ips_deleted_total` | 307 200 | 27 650 |
| Суммарная длительность cleanup | 139,8 с | 0,56 с/запуск |

Средняя длительность cleanup — 0,56 с; 95-й перцентиль около 2,5 с (247 из 248 запусков ≤2,5 с). Пока это не проблема, но массовые DELETE на `domain_stat` — источник bloat.

### Логи

| Период | Строк | Байт |
|---|---:|---:|
| 1 час | 148 585 | 14,15 MB |
| 24 часа | 1 276 225 | **122,05 MB** |

За 1 час:

- 18 233 строки `Received DNS query`;
- 20 921 `Error resolving IPv6`;
- 2 830 `Error resolving IPv4`;
- 2 735 `No IP addresses resolved`;
- 30 `Starting DNS resolution task` (каждые 2 минуты);
- 1 `Running cleanup`.

Файлы логов: collector 14 MB, web-api 38 MB, postgres 8,6 MB; ротация `50m × 5`. 122 MB/сутки — не главный пожиратель SSD, но это заметный вклад в корневой диск и шум.

### Web API

- `HEAD /health` — 29 656 запросов за 11,1 суток (~2 670/сутки, каждые ~32 с).
- `GET /export/telegram` — 20, среднее 0,479 с.
- `GET /export/tiktok` — 10, среднее 0,449 с.
- `GET /export/youtube` — 10, среднее 0,522 с.
- `/api/stats` и `/api/domains` — 0 запросов за окно (UI не использовался; работали только healthcheck и экспорт).

---

## 10. Фактическая конфигурация

### Collector

| Параметр | Значение | Комментарий |
|---|---:|---|
| `resolver.interval_seconds` | **120** | в комментарии написано «5 minutes» — расхождение |
| `resolver.workers` | **100** | батч = `workers × 10 = 1 000` |
| `resolver.timeout_seconds` | 10 | |
| `resolver.max_resolv` | 10 | |
| `resolver.cyclic_resolv` | true | сброс на 2/3 |
| `resolver.resolv_cooldown_mins` | 360 | ветка cooldown недостижима |
| `retention.stats_days` | **7** | в комментарии «30 days» — расхождение |
| `retention.cleanup_interval_hours` | **1** | в комментарии «24 hours» — расхождение |
| `retention.ip_ttl_days` | 3 | |
| `retention.domain_ttl_days` | 14 | |
| `metrics.port` | 8081 | не 9090 |
| `metrics.influxdb.enabled` | true | push раз в 10 с |

Логи показывают батчи по 1 000 доменов каждые 2 минуты: `Found 1000 domains to resolve`.

### PostgreSQL

| Параметр | Значение |
|---|---:|
| `shared_buffers` | 128 MB |
| `effective_cache_size` | 512 MB |
| `work_mem` | 16 MB |
| `maintenance_work_mem` | 128 MB |
| `checkpoint_timeout` | 300 s |
| `checkpoint_completion_target` | 0,9 |
| `max_wal_size` | 1 GB |
| `min_wal_size` | 80 MB |
| `wal_buffers` | 4 MB |
| `wal_compression` | off |
| `full_page_writes` | on |
| `bgwriter_delay` | 200 ms |
| `bgwriter_lru_maxpages` | 100 |
| `autovacuum_naptime` | 60 s |
| `autovacuum_vacuum_cost_limit` | -1 (default) |
| `autovacuum_max_workers` | 3 |
| `autovacuum_vacuum_scale_factor` | 0,2 |
| `autovacuum_analyze_scale_factor` | 0,1 |
| `jit` | **on** (из миграции 000003) |
| `random_page_cost` | 1,1 |
| `effective_io_concurrency` | 1 |
| `track_io_timing` | off |
| `archive_mode` | off |
| `max_connections` | 100 |
| `shared_preload_libraries` | пусто (нет `pg_stat_statements`) |

Настройки `effective_cache_size`, `random_page_cost`, `jit` пришли из миграции `000003_performance_tuning.up.sql`; `shared_buffers`, `work_mem`, `maintenance_work_mem` — дефолтные/из образа.

---

## 11. Что это значит для плана

1. **Приоритет №1 — убрать UPDATE, а не «тюнить» PostgreSQL.** 1,14 млн `domain` и 1,71 млн `ip` UPDATE/сутки при HOT 0,021%/0% — это и есть источник 49,9 ГБ/сутки буферов и 21,2 ГБ/сутки WAL. Этапы 1–2 плана дают основной эффект.
2. **Индексы — вторая по величине проблема.** 93,3% БД — индексы; ~5,8 GB можно удалить/заменить. Это не только место, но и запись: каждый INSERT/UPDATE пишет во все индексы, а autovacuum читает их сотнями миллионов страниц.
3. **Autovacuum надо разгрузить, а не отключать.** 16% записи и 92,7% чтения — это следствие bloat и не-HOT UPDATE. После этапов 1–3 его активность должна упасть в разы; дополнительно помогут per-table настройки и `fillfactor`.
4. **Checkpointer/bgwriter не виноваты, но их можно сделать эффективнее.** `checkpoint_write_time = 89,7%` времени — это не «checkpoint слишком частый», а «грязных страниц слишком много». После снижения UPDATE поможет `shared_buffers` 4 GB, `checkpoint_timeout` 15 мин, `bgwriter_lru_maxpages` 1000.
5. **WAL — не 3 KB/s, а ~21 GB/сутки в этом окне.** Причина — 60,4 млн FPI за 11 суток и `wal_compression = off`. Это не главный драйвер, но после этапов 1–2 стоит включить `wal_compression = zstd`/`lz4` и увеличить `wal_buffers`; проверить эффект.
6. **IPv6-ошибки и логи — отдельная дешёвая победа.** 71% IPv6-lookup'ов без результата и 122 MB/сутки логов; можно не резолвить AAAA для доменов, где он не запрашивался, и убрать лог на каждый пакет.
7. **Конфиг коллектора надо привести в соответствие с комментариями.** `interval_seconds = 120` (не 300), `stats_days = 7` (не 30), cleanup 1 ч (не 24 ч). Это влияет на нагрузку и retention.
8. **Ограничения baseline:** нет `pg_stat_statements` и `track_io_timing`, нет ZFS-метрик хоста. Перед изменениями включить их (этап 0) и повторить срез; иначе не будет видно, какой запрос даёт вклад.

---

## 12. Как повторить замер

### SQL

~~~sql
-- DML и HOT
SELECT relname, n_tup_ins, n_tup_upd, n_tup_hot_upd, n_tup_del,
       n_live_tup, n_dead_tup, autovacuum_count, last_autovacuum
FROM pg_stat_user_tables ORDER BY relname;

-- Размеры
SELECT relname,
       pg_size_pretty(pg_total_relation_size(relid)) AS total,
       pg_size_pretty(pg_relation_size(relid)) AS heap,
       pg_size_pretty(pg_total_relation_size(relid) - pg_relation_size(relid)) AS idx
FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC;

-- Индексы
SELECT relname, indexrelname, idx_scan,
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes ORDER BY relname, pg_relation_size(indexrelid) DESC;

-- Запись/чтение
SELECT * FROM pg_stat_bgwriter;
SELECT * FROM pg_stat_wal;
SELECT * FROM pg_stat_io ORDER BY writes DESC;
SELECT * FROM pg_stat_database WHERE datname = 'dns_collector';

-- После включения pg_stat_statements (2026-11-23)
SELECT calls, total_exec_time, wal_bytes, query
FROM pg_stat_statements ORDER BY wal_bytes DESC LIMIT 20;
~~~

### ОС

~~~bash
grep -E ' (sda|sda1|sdb|sdb1) ' /proc/diskstats
docker stats --no-stream
docker logs --since 24h dns-collector 2>&1 | wc -lc
curl -s http://localhost:8081/metrics
curl -s http://localhost:8080/metrics
~~~

### Критерии сравнения

- `n_tup_upd` по `domain`/`ip` — цель ≤0,2 / ≤0,1 млн/сутки.
- `n_tup_hot_upd / n_tup_upd` по `domain` — ≥90%; по `ip` — допустимо ниже, если абсолютное число UPDATE ≤0,1 млн/сутки.
- `domain_stat` DELETE — 0 (DROP PARTITION).
- `buffers_checkpoint + buffers_clean + buffers_backend` — ≤10 GB/сутки.
- `sdb1` запись — ≤0,6 MB/s (по 5-минутному срезу).
- Размер индексов — ≤1,5 GB суммарно (после удаления/пересоздания).
- Логи — ≤20 MB/сутки.

---

## Приложение. Сырые файлы

- [postgres-snapshot.txt](baseline/2026-11-23/postgres-snapshot.txt) — версия, настройки, размеры, индексы, bgwriter, WAL, pg_stat_io.
- [rates-60s.txt](baseline/2026-11-23/rates-60s.txt) — 60-секундный срез.
- [rates-300s.txt](baseline/2026-11-23/rates-300s.txt) — 5-минутный срез.
- [system-services.txt](baseline/2026-11-23/system-services.txt) — ОС, диски, Docker, конфиги, метрики, логи.
- [collector-metrics.txt](baseline/2026-11-23/collector-metrics.txt) — полный `/metrics` collector.
- [web-api-metrics.txt](baseline/2026-11-23/web-api-metrics.txt) — полный `/metrics` web-api.
