package openai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func newRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.6-codex",
			ChannelType:       1, // ChannelTypeOpenAI
		},
	}
}

func newOverrideRelayInfo(stream bool, enabled bool) *relaycommon.RelayInfo {
	info := newRelayInfo()
	info.IsStream = stream
	info.ChannelOtherSettings.OpenAICodeSafetyClassifierFallback = enabled
	return info
}

func newResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_ChatContentFilter(t *testing.T) {
	body := `{"choices":[{"finish_reason":"content_filter","message":{"role":"assistant","content":null,"refusal":"policy"}}]}`
	if !detectOpenAICodeSafetyClassifierRejection(http.StatusOK, []byte(body)) {
		t.Fatalf("content_filter finish_reason must be detected as rejection")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_ChatRefusal(t *testing.T) {
	body := `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":null,"refusal":"I cannot help with that."}}]}`
	if !detectOpenAICodeSafetyClassifierRejection(http.StatusOK, []byte(body)) {
		t.Fatalf("non-empty refusal must be detected as rejection")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_ResponsesRefusedStatus(t *testing.T) {
	body := `{"status":"refused","output":[{"content":[{"type":"refusal","refusal":"..."}]}]}`
	if !detectOpenAICodeSafetyClassifierRejection(http.StatusOK, []byte(body)) {
		t.Fatalf("responses refused status must be detected as rejection")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_4xxWithPolicyBody(t *testing.T) {
	body := `{"error":{"type":"cyber_policy","message":"blocked"}}`
	if !detectOpenAICodeSafetyClassifierRejection(http.StatusForbidden, []byte(body)) {
		t.Fatalf("403 with cyber_policy body must be detected as rejection")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_Rejects429RateLimit(t *testing.T) {
	body := `{"error":{"type":"rate_limit_exceeded","message":"slow down"}}`
	if detectOpenAICodeSafetyClassifierRejection(http.StatusTooManyRequests, []byte(body)) {
		t.Fatalf("429 rate-limit must NOT be masked as a classifier rejection")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_RejectsAuthError(t *testing.T) {
	body := `{"error":{"message":"Invalid API key"}}`
	if detectOpenAICodeSafetyClassifierRejection(http.StatusUnauthorized, []byte(body)) {
		t.Fatalf("401 auth error without policy signal must NOT be masked")
	}
}

func TestDetectOpenAICodeSafetyClassifierRejection_RejectsNormalResponse(t *testing.T) {
	body := `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}]}`
	if detectOpenAICodeSafetyClassifierRejection(http.StatusOK, []byte(body)) {
		t.Fatalf("normal 200 response must NOT be detected as rejection")
	}
}

func TestShouldOverrideOpenAICodeSafetyClassifierRejection_GatedBySwitch(t *testing.T) {
	info := newOverrideRelayInfo(false, false)
	if shouldOverrideOpenAICodeSafetyClassifierRejection(info) {
		t.Fatalf("override must be off when switch disabled")
	}
	info.ChannelOtherSettings.OpenAICodeSafetyClassifierFallback = true
	if !shouldOverrideOpenAICodeSafetyClassifierRejection(info) {
		t.Fatalf("override should be on when switch enabled")
	}
}

func TestShouldOverrideOpenAICodeSafetyClassifierRejection_RestrictsRelayMode(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.RelayMode = relayconstant.RelayModeEmbeddings
	if shouldOverrideOpenAICodeSafetyClassifierRejection(info) {
		t.Fatalf("override must not apply to non-chat/responses modes")
	}
	info.RelayMode = relayconstant.RelayModeResponses
	if !shouldOverrideOpenAICodeSafetyClassifierRejection(info) {
		t.Fatalf("override should apply to responses mode")
	}
}

func TestMaybeOverrideRejection_RewritesChatContentFilter(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.SetEstimatePromptTokens(128)
	body := `{"choices":[{"finish_reason":"content_filter","message":{"refusal":"blocked"}}]}`
	resp := newResponse(http.StatusOK, body)

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !overrode {
		t.Fatalf("expected override to fire on content_filter body")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status must be 200 after override")
	}
	patched := readAllString(t, resp)
	if !strings.Contains(patched, openAICodeSafetyClassifierChatAllowedText) {
		t.Fatalf("patched body must contain allowed text: %s", patched)
	}
	if !strings.Contains(patched, `"object":"chat.completion"`) {
		t.Fatalf("patched body must be a chat.completion: %s", patched)
	}
	if info.IsStream {
		t.Fatalf("info.IsStream must be reset to false after override")
	}
}

func TestMaybeOverrideRejection_RewritesResponses4xxPolicy(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.RelayMode = relayconstant.RelayModeResponses
	info.SetEstimatePromptTokens(64)
	body := `{"error":{"type":"cyber_policy","message":"blocked"}}`
	resp := newResponse(http.StatusForbidden, body)

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !overrode {
		t.Fatalf("expected override to fire on 4xx cyber_policy")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status must be reset to 200 after override, got %d", resp.StatusCode)
	}
	patched := readAllString(t, resp)
	if !strings.Contains(patched, `"status":"completed"`) {
		t.Fatalf("patched body must be a completed responses object: %s", patched)
	}
	if !strings.Contains(patched, "Allowed by channel OpenAI Codex safety classifier fallback setting.") {
		t.Fatalf("patched body must contain allowed text: %s", patched)
	}
}

func TestMaybeOverrideRejection_PassesThroughNormalResponse(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.SetEstimatePromptTokens(100)
	original := `{"choices":[{"finish_reason":"stop","message":{"content":"hello"}}]}`
	resp := newResponse(http.StatusOK, original)

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrode {
		t.Fatalf("override must NOT fire on a normal response")
	}
	patched := readAllString(t, resp)
	if patched != original {
		t.Fatalf("normal response body must be passed through unchanged: %s", patched)
	}
}

func TestMaybeOverrideRejection_PassesThroughWhenSwitchOff(t *testing.T) {
	info := newOverrideRelayInfo(false, false)
	body := `{"choices":[{"finish_reason":"content_filter","message":{"refusal":"x"}}]}`
	resp := newResponse(http.StatusOK, body)

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrode {
		t.Fatalf("override must NOT fire when switch is off")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status must be unchanged when switch off")
	}
}

func TestMaybeOverrideRejection_DowngradesStreamOnRejection(t *testing.T) {
	info := newOverrideRelayInfo(true, true) // stream request
	info.RelayMode = relayconstant.RelayModeResponses
	info.SetEstimatePromptTokens(50)
	// A 4xx upstream rejection (the primary cyber_policy shape) on a stream
	// request: status code alone identifies it, so the override fires and the
	// stream is downgraded to a non-stream JSON stub.
	body := `{"error":{"type":"cyber_policy","message":"blocked"}}`
	resp := newResponse(http.StatusForbidden, body)
	resp.Header.Set("Content-Type", "text/event-stream")

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !overrode {
		t.Fatalf("override must fire when a 4xx policy body arrives on a stream")
	}
	if info.IsStream {
		t.Fatalf("stream must be downgraded to false after override")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status must be reset to 200 after override")
	}
	patched := readAllString(t, resp)
	if !strings.Contains(patched, `"status":"completed"`) {
		t.Fatalf("stream override must produce a non-stream responses stub: %s", patched)
	}
}

func TestMaybeOverrideRejection_PassesThrough2xxStream(t *testing.T) {
	info := newOverrideRelayInfo(true, true) // stream request, switch on
	info.RelayMode = relayconstant.RelayModeResponses
	// A 2xx streaming success must be forwarded WITHOUT buffering so
	// real-time streaming UX is preserved. Even if the buffered content
	// would contain a content_filter marker, the override does not fire for
	// 2xx streams (we cannot retroactively rewrite flushed events).
	original := "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"
	resp := newResponse(http.StatusOK, original)
	resp.Header.Set("Content-Type", "text/event-stream")

	a := &Adaptor{}
	overrode, err := a.MaybeOverrideRejection(nil, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrode {
		t.Fatalf("override must NOT fire on a 2xx streaming success")
	}
	if !info.IsStream {
		t.Fatalf("stream flag must be preserved for a 2xx stream")
	}
}

func TestBuildOpenAICodeSafetyClassifierChatResponse_HasUsage(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.SetEstimatePromptTokens(128)

	resp, err := buildOpenAICodeSafetyClassifierStubResponse(info)
	if err != nil {
		t.Fatalf("stub response error: %v", err)
	}
	body := readAllString(t, resp)
	if !strings.Contains(body, `"object":"chat.completion"`) {
		t.Fatalf("missing chat object marker: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("missing stop finish_reason: %s", body)
	}
	if !strings.Contains(body, `"total_tokens":144`) {
		t.Fatalf("usage total_tokens should be 128+16: %s", body)
	}
	if !strings.Contains(body, openAICodeSafetyClassifierChatAllowedText) {
		t.Fatalf("missing allowed-text marker: %s", body)
	}
}

func TestBuildOpenAICodeSafetyClassifierResponsesResponse_HasInputOutputTokens(t *testing.T) {
	info := newOverrideRelayInfo(false, true)
	info.RelayMode = relayconstant.RelayModeResponses
	info.SetEstimatePromptTokens(64)

	resp, err := buildOpenAICodeSafetyClassifierStubResponse(info)
	if err != nil {
		t.Fatalf("stub response error: %v", err)
	}
	body := readAllString(t, resp)
	if !strings.Contains(body, `"status":"completed"`) {
		t.Fatalf("missing completed status: %s", body)
	}
	if !strings.Contains(body, `"type":"output_text"`) {
		t.Fatalf("missing output_text content type: %s", body)
	}
	// Responses handlers read input_tokens / output_tokens, so the stub must
	// populate them (not just prompt_tokens / completion_tokens).
	if !strings.Contains(body, `"input_tokens":64`) {
		t.Fatalf("missing input_tokens for responses billing: %s", body)
	}
	if !strings.Contains(body, `"output_tokens":16`) {
		t.Fatalf("missing output_tokens for responses billing: %s", body)
	}
	if !strings.Contains(body, `"total_tokens":80`) {
		t.Fatalf("usage total_tokens should be 64+16: %s", body)
	}
}

func readAllString(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil || resp.Body == nil {
		t.Fatalf("nil response or body")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}
