#!/usr/bin/env python3
"""Analyze 24h dns-collector PostgreSQL monitoring output (stdlib only)."""
import csv
import datetime as dt
import gzip
import json
import os
import sys

csv.field_size_limit(10_000_000)


def fnum(v):
    try:
        return float(v)
    except (TypeError, ValueError):
        return 0.0


def parse_ts(v):
    return dt.datetime.fromisoformat(v.replace("Z", "+00:00"))


def open_text(path):
    if path.endswith(".gz"):
        return gzip.open(path, "rt", newline="", encoding="utf-8")
    return open(path, newline="", encoding="utf-8")


def read_rows(path):
    with open_text(path) as f:
        return list(csv.DictReader(f))


def delta(first, last, col):
    return fnum(last[col]) - fnum(first[col])


def per_day(value, hours):
    return value / hours * 24 if hours else 0


def per_sec(value, hours):
    return value / (hours * 3600) if hours else 0


def pct(a, b):
    return 100.0 * a / b if b else 0


def main():
    data = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else ".")
    out_path = sys.argv[2] if len(sys.argv) > 2 else os.path.join(data, "analysis.json")

    # --- system.csv ---
    sys_rows = sorted(read_rows(os.path.join(data, "system.csv")), key=lambda r: parse_ts(r["ts"]))
    first, last = sys_rows[0], sys_rows[-1]
    start, end = parse_ts(first["ts"]), parse_ts(last["ts"])
    hours = (end - start).total_seconds() / 3600

    syskeys = [
        "checkpoints_timed", "checkpoints_req", "checkpoint_write_time", "checkpoint_sync_time",
        "buffers_checkpoint", "buffers_clean", "buffers_backend", "buffers_alloc",
        "wal_records", "wal_fpi", "wal_bytes", "wal_buffers_full", "wal_write", "wal_sync",
        "blks_read", "blks_hit", "xact_commit", "xact_rollback",
        "tup_inserted", "tup_updated", "tup_deleted", "pgss_dealloc",
    ]
    system = {}
    for k in syskeys:
        d = delta(first, last, k)
        system[k] = {
            "delta": d,
            "per_day": per_day(d, hours),
            "per_sec": per_sec(d, hours),
        }
    buffers = (system["buffers_checkpoint"]["delta"]
               + system["buffers_clean"]["delta"]
               + system["buffers_backend"]["delta"])
    checkpoint_pct = pct(system["checkpoint_write_time"]["delta"],
                         hours * 3600 * 1000)
    cache_hit = pct(system["blks_hit"]["delta"],
                    system["blks_hit"]["delta"] + system["blks_read"]["delta"])

    # --- tables.csv ---
    tbl_rows = read_rows(os.path.join(data, "tables.csv"))
    by_table = {}
    for r in tbl_rows:
        by_table.setdefault(r["relname"], []).append(r)
    tables = {}
    for rel, rows in by_table.items():
        rows.sort(key=lambda r: parse_ts(r["ts"]))
        a, b = rows[0], rows[-1]
        tables[rel] = {
            "first": a["ts"], "last": b["ts"],
            "n_live_tup": fnum(b["n_live_tup"]),
            "n_dead_tup": fnum(b["n_dead_tup"]),
            "ins": delta(a, b, "n_tup_ins"),
            "upd": delta(a, b, "n_tup_upd"),
            "hot": delta(a, b, "n_tup_hot_upd"),
            "del": delta(a, b, "n_tup_del"),
            "upd_per_day": per_day(delta(a, b, "n_tup_upd"), hours),
            "del_per_day": per_day(delta(a, b, "n_tup_del"), hours),
            "hot_pct": pct(delta(a, b, "n_tup_hot_upd"), delta(a, b, "n_tup_upd")),
            "autovacuum_count": fnum(b["autovacuum_count"]),
        }

    # --- disk.csv ---
    disk_rows = read_rows(os.path.join(data, "disk.csv"))
    disk = {}
    for dev in ("sdb1", "sda1"):
        rows = sorted([r for r in disk_rows if r["dev"] == dev], key=lambda r: parse_ts(r["ts"]))
        if not rows:
            continue
        a, b = rows[0], rows[-1]
        w = delta(a, b, "sectors_written") * 512
        rd = delta(a, b, "sectors_read") * 512
        disk[dev] = {
            "written_gb": w / 1e9,
            "read_gb": rd / 1e9,
            "written_gb_day": per_day(w / 1e9, hours),
            "read_gb_day": per_day(rd / 1e9, hours),
            "write_mb_s": per_sec(w / 1e6, hours),
            "read_mb_s": per_sec(rd / 1e6, hours),
        }

    # --- pg_stat_io.csv ---
    io_rows = read_rows(os.path.join(data, "pg_stat_io.csv"))
    by_io = {}
    for r in io_rows:
        key = (r["backend_type"], r["object"], r["context"])
        by_io.setdefault(key, []).append(r)
    io = {}
    for key, rows in by_io.items():
        rows.sort(key=lambda r: parse_ts(r["ts"]))
        a, b = rows[0], rows[-1]
        io["|".join(key)] = {
            "reads": delta(a, b, "reads"),
            "read_time_ms": delta(a, b, "read_time"),
            "writes": delta(a, b, "writes"),
            "write_time_ms": delta(a, b, "write_time"),
            "writebacks": delta(a, b, "writebacks"),
            "extends": delta(a, b, "extends"),
            "fsyncs": delta(a, b, "fsyncs"),
        }

    # --- pg_stat_statements.csv: stream first/last per queryid ---
    first_q, last_q = {}, {}
    nrows = 0
    with open_text(os.path.join(data, "pg_stat_statements.csv.gz")) as f:
        for row in csv.DictReader(f):
            nrows += 1
            qid = row.get("queryid") or "none"
            t = row["ts"]
            if qid not in first_q or t < first_q[qid]["ts"]:
                first_q[qid] = row
            if qid not in last_q or t > last_q[qid]["ts"]:
                last_q[qid] = row
    queries = []
    for qid, a in first_q.items():
        b = last_q[qid]
        calls = delta(a, b, "calls")
        wal = delta(a, b, "wal_bytes")
        total = delta(a, b, "total_exec_time")
        queries.append({
            "queryid": qid,
            "calls": calls,
            "wal_bytes": wal,
            "wal_per_call": wal / calls if calls else 0,
            "total_exec_time_ms": total,
            "mean_exec_time_ms": total / calls if calls else 0,
            "first_ts": a["ts"],
            "last_ts": b["ts"],
            "query": " ".join((b.get("query") or "").split()),
        })

    # --- collector metrics log: first/last values by metric key ---
    def parse_metrics(path):
        first_m, last_m = {}, {}
        cur = None
        if not os.path.exists(path):
            return first_m, last_m
        with open_text(path) as f:
            for line in f:
                if line.startswith("### "):
                    cur = line.strip()[4:]
                    continue
                if not cur or line.startswith("#") or " " not in line:
                    continue
                key, _, val = line.strip().rpartition(" ")
                try:
                    v = float(val)
                except ValueError:
                    continue
                if key not in first_m:
                    first_m[key] = (cur, v)
                last_m[key] = (cur, v)
        return first_m, last_m

    m_first, m_last = parse_metrics(os.path.join(data, "collector-metrics.log"))
    metrics = {}
    for k in [
        'dns_server_messages_received_total{status="valid"}',
        'dns_server_messages_received_total{status="invalid"}',
        'dns_server_domains_received_total{rtype="cache"}',
        'dns_server_domains_received_total{rtype="reply"}',
        'dns_server_new_domains_total',
        'dns_server_processing_duration_seconds_sum',
        'dns_resolver_domains_processed_total{status="success"}',
        'dns_resolver_domains_processed_total{status="no_results"}',
        'dns_resolver_lookups_total{ip_version="ipv4",status="success"}',
        'dns_resolver_lookups_total{ip_version="ipv4",status="error"}',
        'dns_resolver_lookups_total{ip_version="ipv6",status="success"}',
        'dns_resolver_lookups_total{ip_version="ipv6",status="error"}',
        'dns_cleanup_runs_total',
        'dns_cleanup_stats_deleted_total',
        'dns_cleanup_domains_deleted_total',
        'dns_cleanup_ips_deleted_total',
        'dns_cleanup_domain_ips_deleted_total',
        'dns_db_domains_total',
        'dns_db_ips_total',
        'process_cpu_seconds_total',
        'process_resident_memory_bytes',
    ]:
        if k in m_first and k in m_last:
            d = m_last[k][1] - m_first[k][1]
            metrics[k] = {"first": m_first[k][1], "last": m_last[k][1], "delta": d,
                          "per_day": per_day(d, hours)}
    valid = metrics.get('dns_server_messages_received_total{status="valid"}', {}).get("delta", 0)
    proc = metrics.get('dns_server_processing_duration_seconds_sum', {}).get("delta", 0)
    if valid:
        metrics["avg_processing_ms"] = proc / valid * 1000

    # --- build result ---
    queries_sorted = sorted(queries, key=lambda q: q["wal_bytes"], reverse=True)
    total_wal = sum(q["wal_bytes"] for q in queries)
    total_calls = sum(q["calls"] for q in queries)

    result = {
        "data_dir": data,
        "window": {
            "start": first["ts"], "end": last["ts"], "hours": hours,
            "samples": len(sys_rows), "pgss_rows": nrows,
        },
        "system": system,
        "buffers": {
            "total": buffers,
            "gb": buffers * 8192 / 1e9,
            "gb_day": per_day(buffers * 8192 / 1e9, hours),
        },
        "checkpoint_write_pct": checkpoint_pct,
        "cache_hit_pct": cache_hit,
        "tables": tables,
        "disk": disk,
        "io": io,
        "pgss": {
            "queries": len(queries),
            "calls_sum": total_calls,
            "wal_sum": total_wal,
            "top_wal": queries_sorted[:30],
            "top_time": sorted(queries, key=lambda q: q["total_exec_time_ms"], reverse=True)[:20],
            "top_calls": sorted(queries, key=lambda q: q["calls"], reverse=True)[:20],
        },
        "metrics": metrics,
    }

    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(result, f, indent=2, ensure_ascii=False)

    # compact console report
    print(json.dumps({
        "window": result["window"],
        "system": {k: round(v["delta"], 3) for k, v in system.items()},
        "buffers_gb": round(result["buffers"]["gb"], 1),
        "buffers_gb_day": round(result["buffers"]["gb_day"], 2),
        "checkpoint_write_pct": round(checkpoint_pct, 1),
        "cache_hit_pct": round(cache_hit, 2),
        "tables": {k: {"upd": v["upd"], "hot": v["hot"], "del": v["del"],
                       "upd_day": round(v["upd_per_day"]), "hot_pct": round(v["hot_pct"], 3)}
                   for k, v in tables.items()},
        "disk": {k: {kk: round(vv, 3) for kk, vv in v.items()} for k, v in disk.items()},
        "pgss": {"queries": len(queries), "calls": total_calls, "wal_gb": round(total_wal / 1e9, 2)},
        "metrics": {k: round(v["delta"], 2) for k, v in metrics.items() if k in (
            'dns_server_messages_received_total{status="valid"}',
            'dns_resolver_domains_processed_total{status="success"}',
            'dns_resolver_lookups_total{ip_version="ipv6",status="error"}',
            'dns_cleanup_stats_deleted_total',
        )},
    }, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
