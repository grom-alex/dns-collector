# Мониторинг PostgreSQL: 24 часа

**Старт:** 2026-11-23 00:44:16 +06 (2026-11-22 18:44:16 UTC)  
**Окончание:** 2026-11-24 00:47:53 +06 (2026-11-23 18:47:53 UTC)  
**Интервал:** 300 секунд; **286 срезов**  
**Где:** `/opt/dns-collector/monitoring/2026-11-23` на 192.168.0.15  
**Скрипт:** [scripts/pg_monitor_24h.sh](../scripts/pg_monitor_24h.sh)  
**Результаты:** [DB_MONITORING_REPORT.md](DB_MONITORING_REPORT.md)

Мониторинг только читает PostgreSQL, `/proc`, Docker и метрики. БД не изменяется. Секреты не сохраняются: `psql` ходит через локальный сокет внутри контейнера. На стенде файлы лежат как `.csv`/`.log`; в репозитории большие файлы сжаты в `.gz`.

## Что собирается

| Файл | Содержимое | Как использовать |
|---|---|---|
| `pg_stat_statements.csv.gz` | все нормализованные запросы на каждом срезе: `queryid`, `calls`, `rows`, `total_exec_time`, `mean_exec_time`, `wal_bytes`, блоки, `query` | дельты по `queryid` за сутки: топ по WAL/времени/вызовам |
| `tables.csv` | `pg_stat_user_tables`: `n_tup_ins/upd/hot_upd/del`, dead tuples, autovacuum | дельты UPDATE/HOT/DELETE за сутки |
| `pg_stat_io.csv` | `pg_stat_io` по `backend_type/object/context`: чтение, запись, writebacks, fsync | кто именно пишет: checkpointer, bgwriter, autovacuum, backend |
| `system.csv` | `pg_stat_bgwriter`, `pg_stat_wal`, `pg_stat_database`, `pg_stat_statements_info.dealloc` | общие буферы, WAL, транзакции, dealloc |
| `disk.csv` | `/proc/diskstats` по `sda1`/`sdb1`: sectors read/written | физическая запись/чтение, MB/s |
| `docker.csv` | `docker stats`: CPU, RAM, Block I/O контейнеров | вклад postgres/collector/web-api |
| `collector-metrics.log.gz` | сырой `/metrics` collector (порт 8081) | тренды UDP/резолвера/cleanup |
| `web-api-metrics.log.gz` | сырой `/metrics` web-api (8080) | health/экспорт |
| `analysis.json` | машинный результат анализа | цифры для отчёта/сравнения |
| `errors.log` | ошибки сбора | должен быть пустым |
| `monitor.log`, `meta.txt` | статус, старт/конец | диагностика |

## Что важно при анализе

- `pg_stat_statements` включён в 00:35, мониторинг начался в 00:44, поэтому **абсолютные значения** включают первый батч; для суточных ставок считайте **дельту между первым и последним срезом**.
- `pg_stat_statements` нормализует литералы: один шаблон запроса — одна строка; `queryid` стабилен.
- `system.csv` и `tables.csv` — накопительные счётчики; вычитайте значения.
- `disk.csv`: `sectors_written * 512` — байты; делить на длительность окна.
- `pg_stat_io.write_time`/`read_time` — миллисекунды, только при `track_io_timing=on` (включён).
- `errors.log` должен оставаться пустым. Если там есть ошибки — сначала разобраться, потом анализировать.

## Анализ после 24 часов

### 1. Забрать данные

~~~bash
mkdir -p docs/baseline/2026-11-24
scp -i ~/.ssh/id_ed25519_vps -r \
  root@192.168.0.15:/opt/dns-collector/monitoring/2026-11-23 \
  docs/baseline/2026-11-24/

# В репозитории большие файлы сжаты:
cd docs/baseline/2026-11-24/2026-11-23
gzip -9 pg_stat_statements.csv collector-metrics.log web-api-metrics.log
~~~

### 2. DML и HOT

~~~python
import pandas as pd
p = "docs/baseline/2026-11-24/2026-11-23/tables.csv"
df = pd.read_csv(p, parse_dates=["ts"])
first, last = df.ts.min(), df.ts.max()
for t in ["domain", "ip", "domain_stat"]:
    d = df[df.relname == t].sort_values("ts")
    a, b = d.iloc[0], d.iloc[-1]
    print(t,
          "upd", b.n_tup_upd - a.n_tup_upd,
          "hot", b.n_tup_hot_upd - a.n_tup_hot_upd,
          "del", b.n_tup_del - a.n_tup_del,
          "dead", b.n_dead_tup)
print("window, h:", (last - first).total_seconds() / 3600)
~~~

### 3. WAL и буферы

~~~python
import pandas as pd
s = pd.read_csv("docs/baseline/2026-11-24/2026-11-23/system.csv", parse_dates=["ts"]).sort_values("ts")
a, b = s.iloc[0], s.iloc[-1]
h = (b.ts - a.ts).total_seconds() / 3600
for col in ["wal_bytes", "buffers_checkpoint", "buffers_clean", "buffers_backend"]:
    print(col, (b[col] - a[col]) / h)
print("wal GB/day:", (b.wal_bytes - a.wal_bytes) / 1e9 / h * 24)
print("buffers GB/day:", ((b.buffers_checkpoint + b.buffers_clean + b.buffers_backend)
                          - (a.buffers_checkpoint + a.buffers_clean + a.buffers_backend)) * 8192 / 1e9 / h * 24)
~~~

### 4. Топ запросов по WAL

`pg_stat_statements.csv` содержит несколько срезов; нужна дельта по `queryid`:

~~~python
import pandas as pd
p = "docs/baseline/2026-11-24/2026-11-23/pg_stat_statements.csv.gz"
df = pd.read_csv(p, parse_dates=["ts"])
first = df.sort_values("ts").groupby("queryid").first()
last = df.sort_values("ts").groupby("queryid").last()
d = (last[["calls", "total_exec_time", "wal_bytes"]]
      - first[["calls", "total_exec_time", "wal_bytes"]])
d["query"] = last["query"]
print(d.sort_values("wal_bytes", ascending=False).head(20))
~~~

### 5. Диск

~~~python
import pandas as pd
d = pd.read_csv("docs/baseline/2026-11-24/2026-11-23/disk.csv", parse_dates=["ts"])
for dev in ["sdb1", "sda1"]:
    x = d[d.dev == dev].sort_values("ts")
    a, b = x.iloc[0], x.iloc[-1]
    h = (b.ts - a.ts).total_seconds() / 3600
    print(dev, "write MB/s:", (b.sectors_written - a.sectors_written) * 512 / 1e6 / (h * 3600))
~~~

## Остановить досрочно

~~~bash
kill "$(cat /opt/dns-collector/monitoring/2026-11-23/pid)"
~~~

После остановки `meta.txt` может не содержать `end_utc` — это нормально. Данные до последнего среза остаются.

## Ограничения

- Если VM перезагрузится, `nohup`-процесс не восстановится. Проверьте `pid` после ребута.
- Скрипт не собирает `docker logs` (чтобы не раздувать диск); объём логов фиксировался отдельно в baseline.
- `pg_stat_statements.max = 5000`; при `dealloc > 0` в `system.csv` часть запросов вытеснена — увеличьте и перезапустите.
- Скрипт не делает `pg_stat_statements_reset`; окно начинается с 00:35, а не с 00:44.

## Связанные документы

- [DB_STATISTICS.md](DB_STATISTICS.md) — включение и проверка статистики.
- [DB_BASELINE.md](DB_BASELINE.md) — опорный срез до оптимизации.
- [OPTIMIZATION_PLAN.md](OPTIMIZATION_PLAN.md) — этапы 0–2.
