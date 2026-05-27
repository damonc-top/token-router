---
name: model-mapping-audit
description: >-
  Audit model mapping vs upstream returned model names in new-api message logs.
  Use when users ask to verify whether mapped models were actually sent
  upstream and whether upstream returned canonical/aliased/replaced model names
  (e.g. gpt-5.x, codex, -ops suffix cases).
---

# Model Mapping Audit (new-api)

## Scope

This skill audits:

1. What model `new-api` actually sent upstream (`upstream_model_name` + `request_body.model`)
2. What model upstream returned (`response_body` parsed from JSON/SSE)
3. Whether the returned model is the same model, a canonical alias, or a suspicious replacement

Primary table: `message_logs` in `message-log.db`.

## Fast workflow

### Step 1: Read from database safely

Use immutable read-only mode to avoid lock issues:

```bash
rtk sqlite3 'file:message-log.db?mode=ro&immutable=1' "select count(*) from message_logs;"
```

### Step 2: Define mapping and produce final stats

Replace the `mapping` CTE values as needed:

```sql
with mapping(req, mapped) as (
  values
    ('gpt-5.5','gpt-5.5-ops'),
    ('gpt-5.4','gpt-5.4-ops'),
    ('gpt-5.4-mini','gpt-5.4-mini-ops'),
    ('gpt-5.3-codex','gpt-5.3-codex-ops'),
    ('gpt-5.2','gpt-5.3-codex-spark-ops')
),
logs as (
  select
    l.id, m.req, m.mapped, l.response_status,
    cast(l.request_body as text) as req_body,
    cast(l.response_body as text) as body
  from mapping m
  left join message_logs l
    on l.model_name = m.req and l.upstream_model_name = m.mapped
),
first_event as (
  select *,
    case when instr(body, 'data: ') > 0
      then substr(
        substr(body, instr(body, 'data: ')+6),
        1,
        case
          when instr(substr(body, instr(body, 'data: ')+6), char(10)||char(10)) > 0
            then instr(substr(body, instr(body, 'data: ')+6), char(10)||char(10))-1
          else length(substr(body, instr(body, 'data: ')+6))
        end
      )
      else ''
    end as event_json
  from logs
),
extracted as (
  select
    id, req, mapped, response_status,
    case when json_valid(req_body) then json_extract(req_body, '$.model') end as sent_body_model,
    case
      when json_valid(body) then coalesce(
        json_extract(body, '$.model'),
        json_extract(body, '$.response.model'),
        json_extract(body, '$.message.model')
      )
      when json_valid(event_json) then coalesce(
        json_extract(event_json, '$.response.model'),
        json_extract(event_json, '$.message.model'),
        json_extract(event_json, '$.model')
      )
    end as returned_model
  from first_event
),
summary as (
  select
    req, mapped,
    count(id) as sent_count,
    sum(case when sent_body_model = mapped then 1 else 0 end) as request_body_mapped_count,
    sum(case when response_status = 200 then 1 else 0 end) as success_200_count,
    sum(case when response_status <> 200 then 1 else 0 end) as non_200_count,
    sum(case when response_status = 200 and returned_model = mapped then 1 else 0 end) as same_return_count,
    sum(case when response_status = 200 and returned_model is not null and returned_model <> mapped then 1 else 0 end) as different_return_count,
    sum(case when response_status = 200 and returned_model is null then 1 else 0 end) as no_model_count
  from extracted
  group by req, mapped
),
dist as (
  select
    req, mapped,
    coalesce(returned_model, '(no parsed model)') as returned_model,
    count(*) as cnt
  from extracted
  where response_status = 200
  group by req, mapped, coalesce(returned_model, '(no parsed model)')
),
dist_text as (
  select
    req, mapped,
    group_concat(returned_model || ':' || cnt, ', ') as returned_distribution
  from dist
  group by req, mapped
)
select
  s.req as requested_model,
  s.mapped as sent_to_upstream,
  s.sent_count,
  s.request_body_mapped_count,
  s.success_200_count,
  s.non_200_count,
  s.same_return_count,
  s.different_return_count,
  s.no_model_count,
  printf('%.1f%%',
    case when s.success_200_count > 0
      then 100.0 * s.different_return_count / s.success_200_count
      else 0
    end
  ) as diff_rate_of_200,
  d.returned_distribution
from summary s
left join dist_text d on d.req = s.req and d.mapped = s.mapped
order by s.req;
```

Run:

```bash
rtk sqlite3 -header -column 'file:message-log.db?mode=ro&immutable=1' "<PASTE_SQL>"
```

## Interpretation rules

1. Mapping effective:
   `request_body_mapped_count == sent_count` and `upstream_model_name == mapped`.
2. Likely alias normalization:
   `mapped` and `returned_model` differ only by canonical rename (example: `*-ops` -> base model name).
3. Suspicious replacement:
   `returned_model` changes model family/grade (example: `gpt-5.3-codex-ops` -> `gpt-5.4`).
4. Confidence:
   prioritize `response_status = 200` and high `different_return_count`.

## Targeted drill-down

When suspicious replacement exists, sample rows:

```bash
rtk sqlite3 -header -column 'file:message-log.db?mode=ro&immutable=1' "
with logs as (
  select id, request_id, model_name, upstream_model_name, channel_id, request_url, is_stream,
         datetime(created_at, 'unixepoch', 'localtime') as created_at,
         cast(response_body as text) as body
  from message_logs
  where response_status=200
    and model_name='gpt-5.3-codex'
    and upstream_model_name='gpt-5.3-codex-ops'
),
first_event as (
  select *,
    case when instr(body, 'data: ') > 0
      then substr(substr(body, instr(body, 'data: ')+6), 1,
           case when instr(substr(body, instr(body, 'data: ')+6), char(10)||char(10)) > 0
             then instr(substr(body, instr(body, 'data: ')+6), char(10)||char(10))-1
             else length(substr(body, instr(body, 'data: ')+6))
           end)
      else '' end as event_json
  from logs
)
select
  id, request_id, model_name as requested_model, upstream_model_name as sent_to_upstream,
  coalesce(
    case when json_valid(body) then json_extract(body, '$.model') end,
    case when json_valid(body) then json_extract(body, '$.response.model') end,
    case when json_valid(event_json) then json_extract(event_json, '$.response.model') end
  ) as returned_model,
  channel_id, request_url, is_stream, created_at
from first_event
order by id desc
limit 30;
"
```

## Reporting template

Use a single Markdown table (preferred) with these columns:

| 请求模型 (requested_model) | 发给上游模型 (sent_to_upstream) | 发给上游数量 (sent_count) | request_body 确认数 (request_body_mapped_count) | 200 成功 (success_200_count) | 非 200 (non_200_count) | 200 返回模型分布 (returned_distribution) | 不同名比例 (diff_rate_of_200) |
| --- | --- | ---: | ---: | ---: | ---: | --- | ---: |
| **`gpt-5.3-codex`** | **`gpt-5.3-codex-ops`** | 693 | 693 | 654 | 39 | **`gpt-5.4`**:651, **`gpt-5.3-codex-ops`**:2, 未解析:1 | 99.5% |

Formatting rules:

1. Keep one row per `requested_model -> sent_to_upstream`.
2. Highlight every model name with bold inline code: `**\`model-name\`**`.
3. In `returned_distribution`, highlight returned model names too, for example `**\`gpt-5.4\`**:651`.
4. For `returned_distribution`, replace `(no parsed model)` with `未解析`.
5. Keep `diff_rate_of_200` to one decimal place.
6. Add a short risk conclusion after the table:
   alias normalization / suspicious replacement / inconclusive.
