#!/usr/bin/env python3
"""
分析消息日志中的大模型缓存命中情况。

用法:
  python3 analyze_cache.py                          # 分析所有数据
  python3 analyze_cache.py --date 2026-05-17        # 指定日期
  python3 analyze_cache.py --date 2026-05-16,2026-05-17
  python3 analyze_cache.py --user 4                 # 指定用户
  python3 analyze_cache.py --session <session_id>   # 分析某个 session 的所有轮次
  python3 analyze_cache.py --top 20                 # 显示 top N 条缓存断裂
"""

import sqlite3
import json
import re
import argparse
from datetime import datetime
from collections import defaultdict

DB_PATH = "message-log.db"


def extract_usage_from_sse(body: bytes) -> dict | None:
    """从 SSE response body 中提取 message_start 里的 usage 字段。"""
    try:
        text = body.decode("utf-8", errors="replace")
    except Exception:
        return None

    for line in text.splitlines():
        if not line.startswith("data:"):
            continue
        raw = line[5:].strip()
        if '"message_start"' not in raw:
            continue
        try:
            obj = json.loads(raw)
            return obj.get("message", {}).get("usage")
        except json.JSONDecodeError:
            pass
    return None


def extract_session_from_request(body: bytes) -> str | None:
    """从 request body 的 metadata.user_id 中提取 session_id。"""
    try:
        d = json.loads(body)
        meta = d.get("metadata", {}).get("user_id", "")
        if meta:
            try:
                m = json.loads(meta)
                return m.get("session_id")
            except Exception:
                pass
        return None
    except Exception:
        return None


def extract_messages_count(body: bytes) -> int:
    try:
        d = json.loads(body)
        return len(d.get("messages", []))
    except Exception:
        return 0


def load_logs(db_path: str, dates: list[str] | None, user_id: int | None) -> list[dict]:
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cur = conn.cursor()

    where = ["response_status = 200", "is_stream = 1"]
    params = []

    if dates:
        placeholders = ",".join("?" * len(dates))
        where.append(f"date(datetime(created_at, 'unixepoch', 'localtime')) IN ({placeholders})")
        params.extend(dates)

    if user_id:
        where.append("user_id = ?")
        params.append(user_id)

    sql = f"""
        SELECT id, request_id, user_id, token_id, channel_id, model_name,
               request_body, response_body, body_size, created_at
        FROM message_logs
        WHERE {' AND '.join(where)}
        ORDER BY id ASC
    """
    cur.execute(sql, params)
    rows = cur.fetchall()
    conn.close()

    records = []
    for row in rows:
        usage = extract_usage_from_sse(row["response_body"])
        if usage is None:
            continue

        session_id = extract_session_from_request(row["request_body"])
        msg_count = extract_messages_count(row["request_body"])

        records.append({
            "id": row["id"],
            "request_id": row["request_id"],
            "user_id": row["user_id"],
            "model": row["model_name"],
            "session_id": session_id,
            "msg_count": msg_count,
            "body_size": row["body_size"],
            "created_at": row["created_at"],
            "time": datetime.fromtimestamp(row["created_at"]).strftime("%m-%d %H:%M:%S"),
            "input_tokens": usage.get("input_tokens", 0),
            "cache_read": usage.get("cache_read_input_tokens", 0),
            "cache_write": usage.get("cache_creation_input_tokens", 0),
            "output_tokens": usage.get("output_tokens", 0),
        })

    return records


def analyze_overall(records: list[dict]):
    total = len(records)
    if total == 0:
        print("无数据")
        return

    total_input = sum(r["input_tokens"] for r in records)
    total_read = sum(r["cache_read"] for r in records)
    total_write = sum(r["cache_write"] for r in records)
    total_output = sum(r["output_tokens"] for r in records)

    hit_requests = sum(1 for r in records if r["cache_read"] > 0)
    write_requests = sum(1 for r in records if r["cache_write"] > 0)
    cold_requests = sum(1 for r in records if r["cache_read"] == 0 and r["cache_write"] == 0)

    total_effective_input = total_input + total_read + total_write
    hit_rate_by_token = total_read / total_effective_input * 100 if total_effective_input > 0 else 0
    hit_rate_by_req = hit_requests / total * 100

    print("=" * 60)
    print("整体缓存命中分析")
    print("=" * 60)
    print(f"总请求数:           {total}")
    print(f"  有缓存命中:       {hit_requests} ({hit_rate_by_req:.1f}%)")
    print(f"  有缓存写入:       {write_requests}")
    print(f"  完全冷启动:       {cold_requests}")
    print()
    print(f"Token 统计:")
    print(f"  effective input:  {total_effective_input:,}")
    print(f"  cache_read:       {total_read:,}  ({hit_rate_by_token:.1f}% of input)")
    print(f"  cache_write:      {total_write:,}")
    print(f"  direct input:     {total_input:,}")
    print(f"  output:           {total_output:,}")


def analyze_by_session(records: list[dict], session_filter: str | None = None, top_n: int = 20):
    sessions = defaultdict(list)
    for r in records:
        key = r["session_id"] or f"__nosession_{r['user_id']}"
        sessions[key].append(r)

    print("\n" + "=" * 60)
    print("按 Session 分析（缓存断裂检测）")
    print("=" * 60)

    # 如果指定了 session_id，只看那一个
    if session_filter:
        sessions = {k: v for k, v in sessions.items() if session_filter in k}

    results = []
    for sid, rounds in sessions.items():
        rounds = sorted(rounds, key=lambda r: r["id"])
        if len(rounds) < 2:
            continue

        breaks = []
        for i in range(1, len(rounds)):
            prev = rounds[i - 1]
            curr = rounds[i]

            prev_total = prev["cache_read"] + prev["cache_write"] + prev["input_tokens"]
            curr_total = curr["cache_read"] + curr["cache_write"] + curr["input_tokens"]

            # 判断断裂：本轮 cache_read 为 0，但上一轮有缓存（read 或 write）
            prev_had_cache = prev["cache_read"] > 0 or prev["cache_write"] > 0
            curr_no_read = curr["cache_read"] == 0

            # 或者：本轮 cache_read 相对上一轮 total 大幅下降（< 30%）
            drop_ratio = 0.0
            if prev_total > 0 and curr["cache_read"] > 0:
                drop_ratio = curr["cache_read"] / prev_total

            is_break = prev_had_cache and curr_no_read

            breaks.append({
                "round": i,
                "prev_id": prev["id"],
                "curr_id": curr["id"],
                "prev_time": prev["time"],
                "curr_time": curr["time"],
                "prev_cache_read": prev["cache_read"],
                "prev_cache_write": prev["cache_write"],
                "prev_msg_count": prev["msg_count"],
                "curr_cache_read": curr["cache_read"],
                "curr_cache_write": curr["cache_write"],
                "curr_msg_count": curr["msg_count"],
                "is_break": is_break,
                "gap_seconds": curr["created_at"] - prev["created_at"],
            })

        total_rounds = len(rounds)
        break_count = sum(1 for b in breaks if b["is_break"])
        total_read = sum(r["cache_read"] for r in rounds)
        total_eff = sum(r["cache_read"] + r["cache_write"] + r["input_tokens"] for r in rounds)
        session_hit_rate = total_read / total_eff * 100 if total_eff > 0 else 0

        results.append({
            "sid": sid,
            "rounds": rounds,
            "breaks": breaks,
            "total_rounds": total_rounds,
            "break_count": break_count,
            "hit_rate": session_hit_rate,
        })

    # 按断裂次数降序
    results.sort(key=lambda x: (-x["break_count"], -x["total_rounds"]))

    shown = results if session_filter else results[:top_n]

    for res in shown:
        sid = res["sid"]
        print(f"\nSession: {sid[:48] if len(sid) > 48 else sid}")
        print(f"  总轮次: {res['total_rounds']}  断裂次数: {res['break_count']}  缓存命中率: {res['hit_rate']:.1f}%")

        # 打印每轮明细
        header = f"  {'轮':>3}  {'时间':>14}  {'msgs':>5}  {'cache_read':>11}  {'cache_write':>12}  {'input':>8}  {'断裂':>4}"
        print(header)
        print("  " + "-" * (len(header) - 2))
        for i, r in enumerate(res["rounds"]):
            brk = res["breaks"][i - 1] if i > 0 else None
            is_break = "★" if brk and brk["is_break"] else " "
            print(f"  {i+1:>3}  {r['time']:>14}  {r['msg_count']:>5}  "
                  f"{r['cache_read']:>11,}  {r['cache_write']:>12,}  {r['input_tokens']:>8,}  {is_break:>4}")

        # 打印断裂点详情
        for brk in res["breaks"]:
            if not brk["is_break"]:
                continue
            gap = brk["gap_seconds"]
            print(f"\n  ★ 断裂在第 {brk['round']+1} 轮 ({brk['curr_time']})")
            print(f"    上一轮 id={brk['prev_id']}: cache_read={brk['prev_cache_read']:,} cache_write={brk['prev_cache_write']:,} msgs={brk['prev_msg_count']}")
            print(f"    本轮   id={brk['curr_id']}: cache_read={brk['curr_cache_read']:,} cache_write={brk['curr_cache_write']:,} msgs={brk['curr_msg_count']}")
            print(f"    两轮间隔: {gap}s")
            if gap > 300:
                print(f"    → 可能原因: 间隔 {gap}s 超过缓存 TTL (5min ephemeral)")
            elif brk["curr_msg_count"] < brk["prev_msg_count"]:
                print(f"    → 可能原因: messages 数量减少（对话被截断/重置）")
            elif brk["curr_cache_write"] > 0 and brk["prev_cache_write"] > 0:
                print(f"    → 可能原因: system prompt 或前缀内容发生变化，触发新缓存写入")
            else:
                print(f"    → 可能原因: 前缀内容变化或新会话")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--date", help="日期，逗号分隔，如 2026-05-16,2026-05-17")
    parser.add_argument("--user", type=int, help="user_id")
    parser.add_argument("--session", help="指定 session_id（子串匹配）")
    parser.add_argument("--top", type=int, default=20, help="显示 top N 个 session")
    parser.add_argument("--db", default=DB_PATH, help="数据库路径")
    args = parser.parse_args()

    dates = args.date.split(",") if args.date else None

    print(f"加载数据... db={args.db} dates={dates} user={args.user}")
    records = load_logs(args.db, dates, args.user)
    print(f"共加载 {len(records)} 条有效记录\n")

    analyze_overall(records)
    analyze_by_session(records, session_filter=args.session, top_n=args.top)


if __name__ == "__main__":
    main()
