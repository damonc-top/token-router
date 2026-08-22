package claude

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPatchClaudeMessageDeltaUsageDataPreserveUnknownFields(t *testing.T) {
	originalData := `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":53},"vendor_meta":{"trace_id":"trace_001"}}`
	usage := &dto.ClaudeUsage{
		InputTokens:              100,
		CacheReadInputTokens:     30,
		CacheCreationInputTokens: 50,
	}

	patchedData := patchClaudeMessageDeltaUsageData(originalData, usage)

	require.Equal(t, "message_delta", gjson.Get(patchedData, "type").String())
	require.Equal(t, "end_turn", gjson.Get(patchedData, "delta.stop_reason").String())
	require.Equal(t, "trace_001", gjson.Get(patchedData, "vendor_meta.trace_id").String())
	require.EqualValues(t, 53, gjson.Get(patchedData, "usage.output_tokens").Int())
	require.EqualValues(t, 100, gjson.Get(patchedData, "usage.input_tokens").Int())
	require.EqualValues(t, 30, gjson.Get(patchedData, "usage.cache_read_input_tokens").Int())
	require.EqualValues(t, 50, gjson.Get(patchedData, "usage.cache_creation_input_tokens").Int())
}

func TestPatchClaudeMessageDeltaUsageDataZeroValueChecks(t *testing.T) {
	originalData := `{"type":"message_delta","usage":{"output_tokens":53,"input_tokens":9,"cache_read_input_tokens":0}}`
	usage := &dto.ClaudeUsage{
		InputTokens:              100,
		CacheReadInputTokens:     30,
		CacheCreationInputTokens: 0,
	}

	patchedData := patchClaudeMessageDeltaUsageData(originalData, usage)

	require.EqualValues(t, 9, gjson.Get(patchedData, "usage.input_tokens").Int())
	require.EqualValues(t, 30, gjson.Get(patchedData, "usage.cache_read_input_tokens").Int())
	assert.False(t, gjson.Get(patchedData, "usage.cache_creation_input_tokens").Exists())
}

func TestShouldSkipClaudeMessageDeltaUsagePatch(t *testing.T) {
	originGlobalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = originGlobalPassThrough
	})

	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
	assert.True(t, shouldSkipClaudeMessageDeltaUsagePatch(&relaycommon.RelayInfo{}))

	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	assert.True(t, shouldSkipClaudeMessageDeltaUsagePatch(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true}},
	}))
	assert.False(t, shouldSkipClaudeMessageDeltaUsagePatch(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: false}},
	}))
}

func TestBuildMessageDeltaPatchUsage(t *testing.T) {
	t.Run("merge missing fields from claudeInfo", func(t *testing.T) {
		claudeResponse := &dto.ClaudeResponse{Usage: &dto.ClaudeUsage{OutputTokens: 53}}
		claudeInfo := &ClaudeResponseInfo{
			Usage: &dto.Usage{
				PromptTokens: 100,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens:         30,
					CachedCreationTokens: 50,
				},
				ClaudeCacheCreation5mTokens: 10,
				ClaudeCacheCreation1hTokens: 20,
			},
		}

		usage := buildMessageDeltaPatchUsage(claudeResponse, claudeInfo)
		require.NotNil(t, usage)
		require.EqualValues(t, 100, usage.InputTokens)
		require.EqualValues(t, 30, usage.CacheReadInputTokens)
		require.EqualValues(t, 50, usage.CacheCreationInputTokens)
		require.EqualValues(t, 53, usage.OutputTokens)
		require.NotNil(t, usage.CacheCreation)
		require.EqualValues(t, 30, usage.CacheCreation.Ephemeral5mInputTokens)
		require.EqualValues(t, 20, usage.CacheCreation.Ephemeral1hInputTokens)
	})

	t.Run("keep upstream non-zero values", func(t *testing.T) {
		claudeResponse := &dto.ClaudeResponse{Usage: &dto.ClaudeUsage{
			InputTokens:              9,
			CacheReadInputTokens:     7,
			CacheCreationInputTokens: 6,
		}}
		claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{
			PromptTokens: 100,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         30,
				CachedCreationTokens: 50,
			},
		}}

		usage := buildMessageDeltaPatchUsage(claudeResponse, claudeInfo)
		require.EqualValues(t, 9, usage.InputTokens)
		require.EqualValues(t, 7, usage.CacheReadInputTokens)
		require.EqualValues(t, 6, usage.CacheCreationInputTokens)
	})

	t.Run("default aggregate cache creation to 5m when split missing", func(t *testing.T) {
		claudeResponse := &dto.ClaudeResponse{Usage: &dto.ClaudeUsage{
			OutputTokens:             53,
			CacheCreationInputTokens: 50,
		}}
		claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{
				CachedCreationTokens: 50,
			},
		}}

		usage := buildMessageDeltaPatchUsage(claudeResponse, claudeInfo)
		require.NotNil(t, usage)
		require.NotNil(t, usage.CacheCreation)
		require.EqualValues(t, 50, usage.CacheCreation.Ephemeral5mInputTokens)
		require.EqualValues(t, 0, usage.CacheCreation.Ephemeral1hInputTokens)
	})
}

func TestHydrateDeepSeekAnthropicCompatUsageTopLevel(t *testing.T) {
	response := dto.ClaudeResponse{}
	rawResponse := `{"type":"message_delta","usage":{"input_tokens":120,"output_tokens":45,"cache_read_input_tokens":20,"cache_creation_input_tokens":18,"cache_creation":{"ephemeral_5m_input_tokens":8,"ephemeral_1h_input_tokens":6}}}`
	hydrateDeepSeekClaudeUsageFromCompat(&response, rawResponse)

	require.NotNil(t, response.Usage)
	require.EqualValues(t, 120, response.Usage.InputTokens)
	require.EqualValues(t, 45, response.Usage.OutputTokens)
	require.EqualValues(t, 20, response.Usage.CacheReadInputTokens)
	require.EqualValues(t, 18, response.Usage.CacheCreationInputTokens)
	require.NotNil(t, response.Usage.CacheCreation)
	require.EqualValues(t, 8, response.Usage.CacheCreation.Ephemeral5mInputTokens)
	require.EqualValues(t, 6, response.Usage.CacheCreation.Ephemeral1hInputTokens)
}

func TestHydrateDeepSeekAnthropicCompatUsageMessageNested(t *testing.T) {
	response := dto.ClaudeResponse{
		Message: &dto.ClaudeMediaMessage{},
	}
	rawResponse := `{"type":"message_start","message":{"model":"deepseek","usage":{"prompt_tokens":240,"completion_tokens":30,"prompt_cache_hit_tokens":15,"prompt_tokens_details":{"cached_tokens":15}}}}`
	hydrateDeepSeekClaudeUsageFromCompat(&response, rawResponse)

	require.NotNil(t, response.Message)
	require.NotNil(t, response.Message.Usage)
	require.EqualValues(t, 240, response.Message.Usage.InputTokens)
	require.EqualValues(t, 30, response.Message.Usage.OutputTokens)
	require.EqualValues(t, 15, response.Message.Usage.CacheReadInputTokens)
}

func TestApplyDeepSeekAnthropicCompatUsagePreserveExistingValues(t *testing.T) {
	dst := &dto.ClaudeUsage{
		InputTokens:              100,
		OutputTokens:             200,
		CacheReadInputTokens:     30,
		CacheCreationInputTokens: 40,
		CacheCreation: &dto.ClaudeCacheCreationUsage{
			Ephemeral5mInputTokens: 50,
			Ephemeral1hInputTokens: 60,
		},
	}

	src := &deepSeekAnthropicCompatUsage{
		InputTokens:              120,
		OutputTokens:             220,
		CacheReadInputTokens:     45,
		CacheCreationInputTokens: 70,
		CacheCreation: &struct {
			Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
			Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
		}{Ephemeral5mInputTokens: 80, Ephemeral1hInputTokens: 90},
	}

	applyDeepSeekAnthropicCompatUsage(dst, src)

	require.EqualValues(t, 100, dst.InputTokens)
	require.EqualValues(t, 200, dst.OutputTokens)
	require.EqualValues(t, 30, dst.CacheReadInputTokens)
	require.EqualValues(t, 40, dst.CacheCreationInputTokens)
	require.EqualValues(t, 50, dst.CacheCreation.Ephemeral5mInputTokens)
	require.EqualValues(t, 60, dst.CacheCreation.Ephemeral1hInputTokens)
}
