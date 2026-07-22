package claude

import (
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

func stopReasonClaude2OpenAI(reason string) string {
	return relayconvert.StopReasonClaudeToOpenAI(reason)
}

func maybeMarkClaudeRefusal(c *gin.Context, stopReason string) {
	if c == nil {
		return
	}
	if strings.EqualFold(stopReason, "refusal") {
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "claude_stop_reason=refusal")
	}
}

func StreamResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.ChatCompletionsStreamResponse {
	return relayconvert.StreamResponseClaude2OpenAI(claudeResponse)
}

func ResponseClaude2OpenAI(claudeResponse *dto.ClaudeResponse) *dto.OpenAITextResponse {
	return relayconvert.ResponseClaude2OpenAI(claudeResponse)
}

type ClaudeResponseInfo = relayconvert.ClaudeResponseInfo

func cacheCreationTokensForOpenAIUsage(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	openAIUsage := relayconvert.UsageFromClaudeUsage(usage)
	if openAIUsage == nil {
		return 0
	}
	return openAIUsage.PromptTokens - usage.PromptTokens - usage.PromptTokensDetails.CachedTokens
}

func buildOpenAIStyleUsageFromClaudeUsage(usage *dto.Usage) dto.Usage {
	mapped := relayconvert.UsageFromClaudeUsage(usage)
	if mapped == nil {
		return dto.Usage{}
	}
	return *mapped
}

func buildMessageDeltaPatchUsage(claudeResponse *dto.ClaudeResponse, claudeInfo *ClaudeResponseInfo) *dto.ClaudeUsage {
	return relayconvert.BuildMessageDeltaPatchUsage(claudeResponse, claudeInfo)
}

func shouldSkipClaudeMessageDeltaUsagePatch(info *relaycommon.RelayInfo) bool {
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	if info == nil {
		return false
	}
	return info.ChannelSetting.PassThroughBodyEnabled
}

type deepSeekAnthropicCompatUsage struct {
	InputTokens              int `json:"input_tokens"`
	PromptTokens             int `json:"prompt_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	PromptCacheHitTokens     int `json:"prompt_cache_hit_tokens"`
	CachedTokens             int `json:"cached_tokens"`

	InputTokensDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"input_tokens_details"`

	PromptTokensDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`

	CacheCreation *struct {
		Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
		Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

type deepSeekAnthropicCompatResponse struct {
	Usage   *deepSeekAnthropicCompatUsage `json:"usage"`
	Message *struct {
		Usage *deepSeekAnthropicCompatUsage `json:"usage"`
	} `json:"message"`
}

func hydrateDeepSeekClaudeUsageFromCompat(claudeResponse *dto.ClaudeResponse, rawResponse string) {
	if claudeResponse == nil || rawResponse == "" {
		return
	}

	var compat deepSeekAnthropicCompatResponse
	if err := common.UnmarshalJsonStr(rawResponse, &compat); err != nil {
		return
	}

	if compat.Usage != nil {
		if claudeResponse.Usage == nil {
			claudeResponse.Usage = &dto.ClaudeUsage{}
		}
		applyDeepSeekAnthropicCompatUsage(claudeResponse.Usage, compat.Usage)
	}

	if claudeResponse.Message != nil && compat.Message != nil && compat.Message.Usage != nil {
		if claudeResponse.Message.Usage == nil {
			claudeResponse.Message.Usage = &dto.ClaudeUsage{}
		}
		applyDeepSeekAnthropicCompatUsage(claudeResponse.Message.Usage, compat.Message.Usage)
	}
}

func applyDeepSeekAnthropicCompatUsage(dst *dto.ClaudeUsage, src *deepSeekAnthropicCompatUsage) {
	if dst == nil || src == nil {
		return
	}

	if dst.InputTokens == 0 {
		if src.InputTokens > 0 {
			dst.InputTokens = src.InputTokens
		} else if src.PromptTokens > 0 {
			dst.InputTokens = src.PromptTokens
		}
	}

	if dst.OutputTokens == 0 {
		if src.OutputTokens > 0 {
			dst.OutputTokens = src.OutputTokens
		} else if src.CompletionTokens > 0 {
			dst.OutputTokens = src.CompletionTokens
		}
	}

	if dst.CacheReadInputTokens == 0 {
		if src.CacheReadInputTokens > 0 {
			dst.CacheReadInputTokens = src.CacheReadInputTokens
		} else if src.PromptCacheHitTokens > 0 {
			dst.CacheReadInputTokens = src.PromptCacheHitTokens
		} else if src.CachedTokens > 0 {
			dst.CacheReadInputTokens = src.CachedTokens
		} else if src.InputTokensDetails != nil && src.InputTokensDetails.CachedTokens != nil && *src.InputTokensDetails.CachedTokens > 0 {
			dst.CacheReadInputTokens = *src.InputTokensDetails.CachedTokens
		} else if src.PromptTokensDetails != nil && src.PromptTokensDetails.CachedTokens != nil && *src.PromptTokensDetails.CachedTokens > 0 {
			dst.CacheReadInputTokens = *src.PromptTokensDetails.CachedTokens
		}
	}

	if dst.CacheCreationInputTokens == 0 && src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}

	if src.CacheCreation != nil && (src.CacheCreation.Ephemeral5mInputTokens > 0 || src.CacheCreation.Ephemeral1hInputTokens > 0) {
		if dst.CacheCreation == nil {
			dst.CacheCreation = &dto.ClaudeCacheCreationUsage{}
		}
		if src.CacheCreation.Ephemeral5mInputTokens > 0 && dst.CacheCreation.Ephemeral5mInputTokens == 0 {
			dst.CacheCreation.Ephemeral5mInputTokens = src.CacheCreation.Ephemeral5mInputTokens
		}
		if src.CacheCreation.Ephemeral1hInputTokens > 0 && dst.CacheCreation.Ephemeral1hInputTokens == 0 {
			dst.CacheCreation.Ephemeral1hInputTokens = src.CacheCreation.Ephemeral1hInputTokens
		}
	}
}

func patchClaudeMessageDeltaUsageData(data string, usage *dto.ClaudeUsage) string {
	return relayconvert.PatchClaudeMessageDeltaUsageData(data, usage)
}

func FormatClaudeResponseInfo(claudeResponse *dto.ClaudeResponse, oaiResponse *dto.ChatCompletionsStreamResponse, claudeInfo *ClaudeResponseInfo) bool {
	return relayconvert.FormatClaudeResponseInfo(claudeResponse, oaiResponse, claudeInfo)
}

func HandleStreamResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, data string) *types.NewAPIError {
	var claudeResponse dto.ClaudeResponse
	err := common.UnmarshalJsonStr(data, &claudeResponse)
	if err != nil {
		common.SysLog("error unmarshalling stream response: " + err.Error())
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if info != nil && info.ChannelType == constant.ChannelTypeDeepSeek {
		hydrateDeepSeekClaudeUsageFromCompat(&claudeResponse, data)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if claudeResponse.StopReason != "" {
		maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	}
	if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
		maybeMarkClaudeRefusal(c, *claudeResponse.Delta.StopReason)
	}
	if info.RelayFormat == types.RelayFormatClaude {
		FormatClaudeResponseInfo(&claudeResponse, nil, claudeInfo)

		if claudeResponse.Type == "message_start" {
			// message_start, 获取usage
			if claudeResponse.Message != nil {
				info.UpstreamModelName = claudeResponse.Message.Model
				if info.IsModelMapped && claudeResponse.Message.Model != info.OriginModelName {
					if patched, err := sjson.Set(data, "message.model", info.OriginModelName); err == nil {
						data = patched
					}
				}
			}
		} else if claudeResponse.Type == "message_delta" {
			// 确保 message_delta 的 usage 包含完整的 input_tokens 和 cache 相关字段
			// 解决 AWS Bedrock 等上游返回的 message_delta 缺少这些字段的问题
			if !shouldSkipClaudeMessageDeltaUsagePatch(info) {
				data = patchClaudeMessageDeltaUsageData(data, buildMessageDeltaPatchUsage(&claudeResponse, claudeInfo))
			}
		}
		helper.ClaudeChunkData(c, claudeResponse, data)
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		response := StreamResponseClaude2OpenAI(&claudeResponse)

		if !FormatClaudeResponseInfo(&claudeResponse, response, claudeInfo) {
			return nil
		}

		err = helper.ObjectData(c, response)
		if err != nil {
			logger.LogError(c, "send_stream_response_failed: "+err.Error())
		}
	}
	return nil
}

func HandleStreamFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) {
	if claudeInfo.Usage.PromptTokens == 0 {
		//上游出错
	}
	if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
		if common.DebugEnabled {
			common.SysLog("claude response usage is not complete, maybe upstream error")
		}
		// 只补缺失字段，不整份覆盖——保留 message_start 已拿到的 cache 字段
		fallback := service.ResponseText2Usage(c, claudeInfo.ResponseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		if claudeInfo.Usage.CompletionTokens == 0 ||
			(!claudeInfo.Done && fallback.CompletionTokens > claudeInfo.Usage.CompletionTokens) {
			claudeInfo.Usage.CompletionTokens = fallback.CompletionTokens
		}
		if claudeInfo.Usage.PromptTokens == 0 {
			claudeInfo.Usage.PromptTokens = fallback.PromptTokens
		}
		claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens
	}
	if claudeInfo.Usage != nil {
		claudeInfo.Usage.UsageSemantic = "anthropic"
	}
	if claudeInfo.Usage != nil && claudeInfo.Usage.BillingUsage == nil {
		claudeInfo.Usage.BillingUsage = dto.NewClaudeMessagesBillingUsage(buildMessageDeltaPatchUsage(nil, claudeInfo))
	}

	if info.RelayFormat == types.RelayFormatClaude {
		//
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		if info.ShouldIncludeUsage {
			openAIUsage := buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
			response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.UpstreamModelName, openAIUsage)
			err := helper.ObjectData(c, response)
			if err != nil {
				common.SysLog("send final response failed: " + err.Error())
			}
		}
		helper.Done(c)
	}
}

func ClaudeStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var err *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		err = HandleStreamResponseData(c, info, claudeInfo, data)
		if err != nil {
			sr.Stop(err)
		}
	})
	if err != nil {
		return nil, err
	}

	HandleStreamFinalResponse(c, info, claudeInfo)
	return claudeInfo.Usage, nil
}

func HandleClaudeResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, httpResp *http.Response, data []byte) *types.NewAPIError {
	data = maybeApplyClaudeCodeSafetyClassifierFallback(c, info, httpResp, data)

	var claudeResponse dto.ClaudeResponse
	err := common.Unmarshal(data, &claudeResponse)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if info != nil && info.ChannelType == constant.ChannelTypeDeepSeek {
		hydrateDeepSeekClaudeUsageFromCompat(&claudeResponse, string(data))
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	maybeMarkClaudeRefusal(c, claudeResponse.StopReason)
	if claudeInfo.Usage == nil {
		claudeInfo.Usage = &dto.Usage{}
	}
	if claudeResponse.Usage != nil {
		claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
		claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.TotalTokens = claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.UsageSemantic = "anthropic"
		claudeInfo.Usage.BillingUsage = dto.NewClaudeMessagesBillingUsage(claudeResponse.Usage)
		claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Usage.CacheReadInputTokens
		claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Usage.CacheCreationInputTokens
		claudeInfo.Usage.ClaudeCacheCreation5mTokens = claudeResponse.Usage.GetCacheCreation5mTokens()
		claudeInfo.Usage.ClaudeCacheCreation1hTokens = claudeResponse.Usage.GetCacheCreation1hTokens()
	}
	var responseData []byte
	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		openaiResponse := ResponseClaude2OpenAI(&claudeResponse)
		openaiResponse.Usage = buildOpenAIStyleUsageFromClaudeUsage(claudeInfo.Usage)
		responseData, err = common.Marshal(openaiResponse)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatClaude:
		responseData = data
		if info.IsModelMapped && claudeResponse.Model != info.OriginModelName {
			if patched, pErr := sjson.SetBytes(responseData, "model", info.OriginModelName); pErr == nil {
				responseData = patched
			}
		}
	}

	if claudeResponse.Usage != nil && claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}

	service.IOCopyBytesGracefully(c, httpResp, responseData)
	return nil
}

func ClaudeHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	logger.LogDebug(c, "responseBody: %s", responseBody)
	handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody)
	if handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}
