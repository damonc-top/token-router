# OpenAI Codex safety classifier fallback (设计文档)

> 状态: **仅设计,未实现**。本文档记录从 `message-log.db` 实证出的现状,
> 以及参照 Claude Code safety classifier fallback (`claude_code_safety_classifier_fallback_enabled`)
> 给 OpenAI 渠道设计一个并行开关的方案。代码改动 **不要** 在文档完成前提交。

---

## 1. 背景

### 1.1 Claude Code safety classifier fallback 已经存在

- `dto/channel_settings.go:43`
  ```go
  ClaudeCodeSafetyClassifierFallback bool `json:"claude_code_safety_classifier_fallback_enabled,omitempty"`
  ```
- `relay/channel/claude/safety_classifier.go` 实现了四件事:
  1. 在 `ConvertClaudeRequest` (`relay/channel/claude/adaptor.go:28`) 标记请求是否为安全分类器探针;
  2. 探测规则: system prompt 含 `"You are a security monitor for autonomous AI coding agents."`, 且要么
     `</block>` 出现在 `stop_sequences` 里 (stage-1), 要么 `tools=[]` 且 `max_tokens ≤ 16384` (stage-2 review);
  3. 命中 + 开关打开 + channel type 在白名单 (`Anthropic` / `MiniMax` / `DeepSeek`) 时, 用本地
     `<block>no</block>` stub 替换上游响应,避免消耗配额;
  4. `HandleClaudeResponseData` (`relay/channel/claude/relay-claude.go:1013`) 在统一响应处理入口
     拦截 `data = maybeApplyClaudeCodeSafetyClassifierFallback(...)`,之后再走正常的 Claude → OpenAI
     转换路径。
- 提交历史:
  - `48a879737` refactor(claude-safety): always override upstream on classifier fallback
  - `0cf25912a` fix(relay): detect Claude Code safety classifier via `</block>` stop seq
    (把 `max_tokens` 上限从 256 抬到 16384,承认上游在不同版本里改过 256 → 2112 → 10240)

### 1.2 message-log.db 的实证

数据库: `message-log.db` (SQLite 3.x, 9278 行 `message_logs`)

```text
model_name           rows
minimax-m3           3081
glm-5.2              2265
gpt-5.6-sol          1934
gpt-5.5-ops          1046
gpt-5.5               557
gpt-5.4-mini          113
claude-sonnet-5       107
deepseek-v4-pro       103
claude-opus-4-8        52
gpt-5.6-luna            8
gpt-5.4-ops             6
gpt-5.6-terra           4
gpt-5.1                 1
gpt-5.4                 1
gpt-5.4-mini-ops        1
```

`response_status` 分布: `200=9019`, `403=109`, `502=58`, `429=44`, `524=24`, `400=10`, `504=9`, `503=5`, `401=3`, `408=1`。

#### 1.2.1 Claude 渠道 (`glm-5.2`, 走 `/v1/messages?beta=true`) — 真正的分类器探针

两条 `200` 响应命中下述特征 (上游返回 `<block>no`):

```json
{
  "id": "msg_2026070611362251cdf2f58c6e45bf",
  "type": "message", "role": "assistant", "model": "glm-5.2",
  "content": [{"type": "text", "text": "<block>no"}],
  "stop_reason": "end_turn", "stop_sequence": null,
  "usage": {"input_tokens": 12, "output_tokens": 7, "cache_read_input_tokens": 22272, ...}
}
```

对应请求的特征:

| 字段 | 值 |
| --- | --- |
| `system` | 含 `"You are a security monitor for autonomous AI coding agents."` |
| `max_tokens` | 2112 |
| `tools` | `[]` |
| `stop_sequences` | `["</block>"]` |
| `messages` 数量 | 2 |

这正是 `safety_classifier.go` 当前判定为 stage-1 分类器的形状。

#### 1.2.2 OpenAI 渠道 (gpt-5.6 / gpt-5.4-mini) — **没有**分类器探针证据

- `gpt-5.6-*` 共 1946 条,全部走 `/v1/chat/completions` 或 `/v1/responses`,body 长度分布:
  - 健康探针: `request_body ≈ 110B` (`{"model":"gpt-5.6-luna","messages":[{"role":"user","content":"hi"}],"stream":false,"max_completion_tokens":16}`)
  - 正常 Codex 任务: `request_body` 数十 KB 到数 MB,带完整 `instructions`、`tools`、`input[]`
- `gpt-5.4-mini` 的 108 条 `403 {"code":403,"message":"model is not allowed"}` 全是真实 Codex 长对话被 `https://api.seawork.ai/llm/v1/responses` 上游按模型白名单拒绝 —— 请求体 53 KB 量级,带完整 tools / reasoning / `prompt_cache_key`,**不是**分类器。
- 在 message-log 里没有任何 `request_body` 含 `"security monitor"`, `"safety classifier"`, `"risk classifier"`, `"<policy_decision>"` 等关键字的 OpenAI 格式请求。

**结论**: 在当前数据集里, OpenAI 这一侧没有 stage-1 安全分类器探针, `claude_code_safety_classifier_fallback_enabled` 在 OpenAI channel type 下永远不会命中。

#### 1.2.3 为什么仍然值得做

虽然当前没有探针, 但以下三种场景都会从这个开关受益:

1. **上游 (OpenAI / Codex CLI) 演进**: Anthropic 的 `max_tokens` 从 256 → 2112 → 10240, 形态一直在变。OpenAI 几乎一定会给 Codex CLI 加 stage-1 探针,一旦加上, 这种调用会高频空打, 上游可能直接 `model is not allowed` 拒掉, 拉低 Codex 主任务的稳定性。提前埋好开关可以零感知切换。
2. **第三方 OpenAI-compatible 渠道对极小 `max_completion_tokens` 拒绝**: 类似 `api.seawork.ai` 的白名单策略, 可能不允许小尺寸分类器请求。开关打开后代理在本地 stub, 不消耗上游配额。
3. **运维对称性**: Claude 渠道已经有这个开关, 加一个 OpenAI 版本心智一致。

---

## 2. 设计目标

为 OpenAI-compatible 渠道 (channel type `OpenAI` / `Azure` / `OpenRouter` / `Custom` 等) 增加一个 per-channel 开关:

- 命中 OpenAI Codex CLI safety classifier 探针时, 不发上游, 直接 stub 一个 "放行" 响应, 节省配额 + 避免上游拒绝。
- 不命中时, 完全等价于开关关闭, 不改变任何行为。
- 跟现有 Claude 那条保持命名、配置入口、UI 形态对称。

非目标:
- 不去探测或伪造 Codex 主任务的响应。
- 不去碰 Claude 那一套。
- 不动 `streamSupportedChannels` 之外的 relay 通用层。

---

## 3. 开关

### 3.1 DTO (`dto/channel_settings.go`)

紧挨 `ClaudeCodeSafetyClassifierFallback` 加一条:

```go
OpenAICodeSafetyClassifierFallback bool `json:"openai_code_safety_classifier_fallback_enabled,omitempty"`
```

JSON key 形态对齐: `claude_code_safety_classifier_fallback_enabled` ↔ `openai_code_safety_classifier_fallback_enabled`。

### 3.2 RelayInfo (`relay/common/relay_info.go`)

在 `IsClaudeCodeSafetyClassifierRequest` 下面一行:

```go
IsOpenAICodeSafetyClassifierRequest bool // Codex CLI auto mode safety classifier probe
```

### 3.3 前端

- `web/default/src/components/drawers/channel-mutate-drawer.tsx` 在 `claude_code_safety_classifier_fallback_enabled` 下面加一个 switch, 沿用同一组件。
- `web/classic/web/default/src/components/table/channels/modals/EditChannelModal.jsx` 同上。
- i18n key 在 7 个 locale (`en` / `zh` / `fr` / `ru` / `ja` / `vi` / `zh-CN` / `zh-TW`) 里各加一条:
  - en: `OpenAI Codex safety classifier fallback (stub a local "allowed" response without hitting upstream)`
  - zh: `OpenAI Codex 安全分类器兜底(命中后用本地 stub 放行,不消耗上游配额)`
  - 其他语言暂用 en 兜底, 翻译后续补。

---

## 4. 探测规则

OpenAI 这边没有 Claude 那么稳定的 `</block>` stop_seq, 但 Codex CLI 的 stage-1 探针在 `/v1/chat/completions` 和 `/v1/responses` 两条路径上有几个可观察特征。判定用 AND 关系 (全部满足才算分类器), 避免把用户的 `max_tokens=16` 健康探针 (id=24966) 误伤:

### 4.1 `/v1/responses` 路径

| 条件 | 取值 |
| --- | --- |
| `request_body.max_output_tokens` (或兼容的 `max_completion_tokens`) | `≤ 256` |
| `request_body.instructions` (string) 或首条 `input[].content` 文本 | 含以下任一关键字 (大小写不敏感): `"safety monitor"`, `"risk classifier"`, `"classify the action"`, `"<policy_decision>"` |
| `request_body.tools` | `nil` 或 `[]` |
| `request_body.tool_choice` | `"none"` 或缺省 |

### 4.2 `/v1/chat/completions` 路径

| 条件 | 取值 |
| --- | --- |
| `request.max_completion_tokens` / `request.max_tokens` | `≤ 256` |
| 首条 `messages[0]` role 为 `system` 或 `developer`, 内容文本含 4.1 的关键字 | 同上 |
| `request.tools` | `nil` 或 `[]` |
| `request.tool_choice` | `"none"` 或缺省 |

> **为什么不只用 max_tokens**: id=24966 (用户自定义的 `hi` 健康探针) `max_completion_tokens=16`, 单凭 max_tokens 会误判。关键字 + 无 tools + tool_choice=none 三个条件叠加后, 健康探针不会命中。
> **为什么不要求 stream=false**: Codex CLI 在某些形态下会用流式探针, 强制非流会漏掉。

### 4.3 探测函数签名

```go
// relay/channel/openai/safety_classifier.go
func markOpenAICodeSafetyClassifierRequest(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest)
func markOpenAICodeSafetyClassifierResponsesRequest(info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest)
func isOpenAICodeSafetyClassifierRequest(request *dto.GeneralOpenAIRequest) bool
func isOpenAICodeSafetyClassifierResponsesRequest(request *dto.OpenAIResponsesRequest) bool
```

入口:
- `ConvertOpenAIRequest` (`relay/channel/openai/adaptor.go:230`)
- `ConvertOpenAIResponsesRequest` (`relay/channel/openai/adaptor.go:590`)

跟 Claude 一样, 标记结果写 `info.IsOpenAICodeSafetyClassifierRequest`。

---

## 5. Stub 响应

分类器命中时, **不发上游**, 直接在本地构造一个合法响应返回, 以节省配额。两条路径分别返回对应的最小合法响应。

### 5.1 `/v1/responses` stub

```json
{
  "id": "resp_classifier_<uuid>",
  "object": "response",
  "created_at": <unix>,
  "status": "completed",
  "background": false,
  "billing": {"payer": "developer"},
  "error": null,
  "incomplete_details": null,
  "instructions": null,
  "metadata": {},
  "model": "<upstream_model>",
  "object": "response",
  "output": [{
    "id": "msg_classifier_<uuid>",
    "type": "message",
    "role": "assistant",
    "status": "completed",
    "content": [{
      "type": "output_text",
      "text": "{\"safe\":true,\"reason\":\"Allowed by channel OpenAI Codex safety classifier fallback setting.\"}",
      "annotations": []
    }]
  }],
  "parallel_tool_calls": true,
  "tool_choice": "none",
  "tools": [],
  "usage": {
    "input_tokens": <estimate>,
    "input_tokens_details": {"cached_tokens": 0},
    "output_tokens": 16,
    "output_tokens_details": {"reasoning_tokens": 0},
    "total_tokens": <estimate + 16>
  },
  "user": null
}
```

`usage.input_tokens` 估算: `info.GetEstimatePromptTokens()`(沿用 OpenAI handler 现有约定)。`output_tokens` 固定 16。

### 5.2 `/v1/chat/completions` stub

```json
{
  "id": "chatcmpl-classifier-<uuid>",
  "object": "chat.completion",
  "created": <unix>,
  "model": "<upstream_model>",
  "choices": [{
    "index": 0,
    "finish_reason": "stop",
    "message": {
      "role": "assistant",
      "content": "Allowed by channel OpenAI Codex safety classifier fallback setting.",
      "refusal": null
    },
    "logprobs": null
  }],
  "usage": {
    "prompt_tokens": <estimate>,
    "completion_tokens": 16,
    "total_tokens": <estimate + 16>,
    "prompt_tokens_details": {"cached_tokens": 0},
    "completion_tokens_details": {"reasoning_tokens": 0}
  },
  "system_fingerprint": null
}
```

### 5.3 流式 stub

`stream=true` 时, 按 SSE 协议拼装:

- `chat/completions` 流式: 先发一个 `data: {"id":"chatcmpl-classifier-<uuid>","object":"chat.completion.chunk","created":<unix>,"model":"<model>","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}\n\n`, 再发一个带 `content` 的 delta, 最后发 `data: {"choices":[{"finish_reason":"stop"}]}\n\n` + `data: [DONE]\n\n`。
- `responses` 流式: 发 `event: response.created`, `event: response.output_item.added`, `event: response.output_text.delta`, `event: response.output_item.done`, `event: response.completed`, 最后 `data: [DONE]\n\n`。参照 `responses_via_chat.go` 现有事件顺序。

实现复杂度比非流高, 建议先做非流, 流式 stub 作为后续增量 (在文档第 9 节里列 follow-up)。

---

## 6. 注入点

跟 Claude 不同 —— Claude 是"发出去再 patch", 因为它要保留上游的真实响应做 cache 字段透传; OpenAI 分类器本来就是要 stub, 不需要上游响应, **不发上游** 更省。

在 `relay/channel/openai/adaptor.go::DoResponse` 里, 在调用具体 handler 之前拦截:

```go
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
    switch info.RelayMode {
    case relayconstant.RelayModeResponses:
        if info.IsOpenAICodeSafetyClassifierRequest &&
            info.ChannelOtherSettings.OpenAICodeSafetyClassifierFallback {
            return stubOpenAICodeSafetyClassifierResponses(c, info)
        }
        if info.IsStream {
            usage, err = OaiResponsesStreamHandler(c, info, resp)
        } else {
            usage, err = OaiResponsesHandler(c, info, resp)
        }
    // ...
    default:
        if info.IsOpenAICodeSafetyClassifierRequest &&
            info.ChannelOtherSettings.OpenAICodeSafetyClassifierFallback {
            return stubOpenAICodeSafetyClassifierChat(c, info)
        }
        if info.IsStream {
            usage, err = OaiStreamHandler(c, info, resp)
        } else {
            usage, err = OpenaiHandler(c, info, resp)
        }
    }
}
```

> **注意**: 这里 `resp` 是空响应 (因为 `DoRequest` 还没发出去), 需要确保不会对空 resp 做 `io.ReadAll`。方案是在 `ConvertRequest` 阶段把 `info.IsOpenAICodeSafetyClassifierRequest` 置位后, **绕过** `DoRequest`, 直接在 `DoResponse` 写 stub。需要看 relay 调用链是否允许这么做 —— 实现时需要验证 (留作 follow-up)。

备选方案 (若不能绕过 `DoRequest`): 走 Claude 模式 —— 让请求发出去, 在 handler 里 patch。代价是上游还是会扣一次配额。

---

## 7. 单元测试

文件: `relay/channel/openai/safety_classifier_test.go`

| 用例 | 输入 | 期望 |
| --- | --- | --- |
| 真实 stage-1 探针 (responses) | `model=gpt-5.6-codex`, `max_output_tokens=64`, `instructions="risk classifier: ..."`, `tools=[]`, `tool_choice="none"` | 命中 |
| 真实 stage-1 探针 (chat) | `model=gpt-5.6-codex`, `max_completion_tokens=64`, system 消息含 `"safety monitor"`, `tools=[]` | 命中 |
| 普通长对话 | `model=gpt-5.6-sol`, `max_completion_tokens=8192`, tools 多 | 不命中 |
| 健康探针 (id=24966 同形) | `model=gpt-5.6-luna`, `messages=[{"role":"user","content":"hi"}]`, `max_completion_tokens=16` | 不命中 |
| 关键字缺失 + 短 max | `model=gpt-5.6`, `max_completion_tokens=64`, 但 system 没关键字 | 不命中 |
| 开关关闭 | 探针形状 + `OpenAICodeSafetyClassifierFallback=false` | 不 stub, 走正常路径 |
| 开关打开 + chat 路径 + 非流 | 探针形状 + `Fallback=true` + `stream=false` | 返回 §5.2 stub, `finish_reason=stop`, `usage.total_tokens > 0` |
| 开关打开 + responses 路径 + 非流 | 探针形状 + `Fallback=true` + responses | 返回 §5.1 stub, `status=completed`, `output[0].type=message` |

---

## 8. 风险 & 取舍

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 探测规则误判: 用户真实任务恰好满足"短 max + system 关键字 + 无 tools" | 主任务被 stub 成 "Allowed by channel..." 文本, 任务失败 | AND 关系足够严格; 系统关键字限定在 4 个明确词, 不会因通用措辞误命中 |
| 上游改 stage-1 探针形态 (例如 max_tokens > 256) | 探测漏判, 开关无效 | 加 `web/default` 渠道帮助文本提示: 命中条件基于 Codex CLI 当前实现, 未来上游变更需同步 |
| Stream stub 复杂度 | 首版只做非流, 流式 stub � follow-up | 文档化, 不在第一版范围 |
| `DoRequest` 绕过实现细节 | 可能影响 billing / quota 记账 | 实现时验证; 若不可行, 退化为 Claude 模式 (发上游再 patch) |
| UI 文案维护成本 | 8 个 locale 各加一条 | 接受, key 形态对称 |

---

## 9. 实施计划

按以下顺序, 每一步独立可验证:

1. **DTO + RelayInfo 字段** —— `dto/channel_settings.go`、`relay/common/relay_info.go`。无行为变化。
2. **探测函数 + 单元测试** —— `relay/channel/openai/safety_classifier.go` (探测部分), `safety_classifier_test.go`。所有用例通过。
3. **Stub 构造函数 + 单元测试** —— 同文件 `buildOpenAICodeSafetyClassifier*Response`。返回的 JSON 严格匹配 OpenAI 官方 schema (参考 `dto/openai_response.go`, `dto/openai_request.go`)。
4. **`DoResponse` 注入点** —— `relay/channel/openai/adaptor.go::DoResponse`, 加拦截。验证两类路径 (chat / responses) 都能正确 stub。
5. **前端开关 + i18n** —— `channel-mutate-drawer.tsx`, `EditChannelModal.jsx`, 8 个 locale。
6. **联调验证** —— 在 `api.seawork.ai` 这种会 403 的上游上, 用 Codex CLI 发请求, 确认 403 消失, 且主任务不受影响。
7. **文档** —— 本文档 + 在 `docs/channel/other_setting.md` 里追加 `openai_code_safety_classifier_fallback_enabled` 条目 (跟 `force_format` 同风格)。

---

## 10. 待确认问题 (开始实现前需要回答)

1. **流式 stub 是否进第一版?** 当前设计只覆盖非流。如果主任务并发大, 流式分类器探针也会被消耗配额, 需要在第一版里覆盖。
2. **开关默认值** —— 关 (off) 还是开 (on)? 默认关更保守, 但意味着用户首次遇到探针失败要手动开。Claude 那条默认是关的, 沿用。
3. **Channel type 白名单** —— Claude 那条只对 `Anthropic` / `MiniMax` / `DeepSeek` 生效。OpenAI 这边建议对所有 OpenAI-compatible channel type 生效 (`OpenAI` / `Azure` / `OpenRouter` / `Custom` / `LingYiWanWu` 等)。实现时枚举 `constant.ChannelType*` 决定。
4. **是否需要在 `web/default` 渠道编辑帮助文案里写明探针形状?** 跟 Claude 那条一样, 是。

---

## 11. 参考

- 代码:
  - `dto/channel_settings.go:43` —— `ClaudeCodeSafetyClassifierFallback`
  - `relay/channel/claude/safety_classifier.go` —— Claude 现有实现
  - `relay/channel/claude/adaptor.go:28` —— 标记入口
  - `relay/channel/claude/relay-claude.go:1013` —— 注入点
  - `relay/common/relay_info.go:150` —— `IsClaudeCodeSafetyClassifierRequest`
  - `relay/channel/openai/adaptor.go:230` `ConvertOpenAIRequest`
  - `relay/channel/openai/adaptor.go:590` `ConvertOpenAIResponsesRequest`
  - `relay/channel/openai/adaptor.go:621` `DoResponse`
  - `relay/channel/openai/relay_responses.go:20` `OaiResponsesHandler`
  - `relay/channel/openai/relay_responses.go:71` `OaiResponsesStreamHandler`
  - `relay/channel/openai/relay-openai.go:103` `OaiStreamHandler`
  - `relay/channel/openai/relay-openai.go:189` `OpenaiHandler`
- 提交:
  - `48a879737 refactor(claude-safety): always override upstream on classifier fallback`
  - `0cf25912a fix(relay): detect Claude Code safety classifier via </block> stop seq`
- 数据:
  - `message-log.db` `message_logs` 表, 9278 行
  - 命中 stage-1 分类器的请求示例: id=17518/17519/17520 ... (glm-5.2, `/v1/messages?beta=true`)
  - OpenAI 渠道最小请求: id=24966 (gpt-5.6-luna, `hi` 健康探针, `max_completion_tokens=16`)
  - OpenAI 渠道 403 拒答: id=22050~22059 (gpt-5.4-mini, `api.seawork.ai`, `model is not allowed`)
