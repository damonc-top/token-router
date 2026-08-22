package claudemessages

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamSanitizerValidStreamIsIdentity(t *testing.T) {
	inputs := []string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","content":[]}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call_1","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"ls\"}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":8}}`,
		`{"type":"message_stop"}`,
	}

	out := runClaudeStreamSanitizer(t, inputs)
	require.Len(t, out, len(inputs))
	for i, ev := range out {
		assert.Equal(t, inputs[i], ev.Data, "event %d data", i)
		assert.Equal(t, eventType(inputs[i]), ev.Response.Type)
	}
}

func TestClaudeStreamSanitizerMinimaxOverlappingToolAndText(t *testing.T) {
	inputs := []string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"minimax-m3","content":[]}}`,
		`{"content_block":{"type":"text","text":""},"type":"content_block_start","index":0}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"跑超时了"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_b8ada0a970577d26","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\": \"cat \\\"C:/"}}`,
		`{"index":0,"content_block":{"text":"","type":"text"},"type":"content_block_start"}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"进度："}}`,
		`{"index":0,"delta":{"type":"input_json_delta","partial_json":"Users/afeng/AppData"},"type":"content_block_delta"}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
		`{"type":"message_stop"}`,
	}

	out := runClaudeStreamSanitizer(t, inputs)
	got := summarizeClaudeSanitized(out)
	assert.Equal(t, []string{
		"message_start",
		"content_block_start:0:text",
		"content_block_delta:0:text_delta",
		"content_block_stop:0",
		"content_block_start:0:tool_use",
		"content_block_delta:0:input_json_delta",
		"content_block_start:1:text",
		"content_block_delta:1:text_delta",
		"content_block_delta:0:input_json_delta",
		"content_block_stop:0",
		"content_block_stop:1",
		"message_delta",
		"message_stop",
	}, got)

	assert.Equal(t, "call_b8ada0a970577d26", out[4].Response.ContentBlock.Id)
	assert.Equal(t, "Bash", out[4].Response.ContentBlock.Name)
	require.NotNil(t, out[5].Response.Delta)
	require.NotNil(t, out[5].Response.Delta.PartialJson)
	assert.Equal(t, `{"command": "cat \"C:/`, *out[5].Response.Delta.PartialJson)
	require.NotNil(t, out[8].Response.Delta)
	require.NotNil(t, out[8].Response.Delta.PartialJson)
	assert.Equal(t, "Users/afeng/AppData", *out[8].Response.Delta.PartialJson)
}

func TestClaudeStreamSanitizerDropsOrphanInputJsonDelta(t *testing.T) {
	inputs := []string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
	}
	out := runClaudeStreamSanitizer(t, inputs)
	assert.Equal(t, []string{
		"content_block_start:0:text",
		"content_block_delta:0:text_delta",
	}, summarizeClaudeSanitized(out))
}

func TestClaudeStreamSanitizerClosesOpenBlocksBeforeMessageDelta(t *testing.T) {
	inputs := []string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"Bash","input":{}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
	}
	out := runClaudeStreamSanitizer(t, inputs)
	assert.Equal(t, []string{
		"content_block_start:0:tool_use",
		"content_block_start:1:text",
		"content_block_delta:0:input_json_delta",
		"content_block_stop:1",
		"content_block_stop:0",
		"message_delta",
	}, summarizeClaudeSanitized(out))
}

func TestClaudeStreamSanitizerStopsTextBeforeToolReuse(t *testing.T) {
	inputs := []string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
	}
	out := runClaudeStreamSanitizer(t, inputs)
	assert.Equal(t, []string{
		"content_block_start:0:text",
		"content_block_delta:0:text_delta",
		"content_block_stop:0",
		"content_block_start:0:tool_use",
		"content_block_delta:0:input_json_delta",
	}, summarizeClaudeSanitized(out))
}

func TestClaudeStreamSanitizerKeepsInterleavedThinkingIndexes(t *testing.T) {
	inputs := []string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan"}}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"hi"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_stop","index":0}`,
	}
	out := runClaudeStreamSanitizer(t, inputs)
	require.Len(t, out, len(inputs))
	for i, ev := range out {
		assert.Equal(t, inputs[i], ev.Data, "event %d data", i)
	}
}

func runClaudeStreamSanitizer(t *testing.T, inputs []string) []ClaudeSanitizedEvent {
	t.Helper()
	s := NewClaudeStreamSanitizer()
	out := make([]ClaudeSanitizedEvent, 0, len(inputs))
	for _, in := range inputs {
		var ev dto.ClaudeResponse
		require.NoError(t, common.UnmarshalJsonStr(in, &ev))
		out = append(out, s.Process(&ev, in)...)
	}
	return out
}

func summarizeClaudeSanitized(events []ClaudeSanitizedEvent) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		switch ev.Response.Type {
		case "content_block_start":
			typ := ""
			if ev.Response.ContentBlock != nil {
				typ = ev.Response.ContentBlock.Type
			}
			out = append(out, "content_block_start:"+itoa(ev.Response.GetIndex())+":"+typ)
		case "content_block_delta":
			dt := ""
			if ev.Response.Delta != nil {
				dt = ev.Response.Delta.Type
			}
			out = append(out, "content_block_delta:"+itoa(ev.Response.GetIndex())+":"+dt)
		case "content_block_stop":
			out = append(out, "content_block_stop:"+itoa(ev.Response.GetIndex()))
		default:
			out = append(out, ev.Response.Type)
		}
	}
	return out
}

func eventType(data string) string {
	var ev dto.ClaudeResponse
	_ = common.UnmarshalJsonStr(data, &ev)
	return ev.Type
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
