package plugins_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadVtrixPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("vtrix")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "vtrix"})
	require.NoError(t, err)
	return plugin
}

func vtrixObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, common.Unmarshal(encoded, &result))
	return result
}

func vtrixRequest(model string, params map[string]any) map[string]any {
	return map[string]any{"model": model, "input": []any{map[string]any{"params": params}}}
}

func vtrixContext(model string, body map[string]any) map[string]any {
	return map[string]any{
		"model": model, "upstreamModel": model, "baseUrl": "https://cloud.vtrix.ai",
		"apiKey": "test-key", "requestBody": body, "usagePurpose": "facts",
	}
}

func TestVtrixNativeRequestsPreserveProviderParameters(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	tests := []struct {
		model  string
		params map[string]any
	}{
		{"wan27_image", map[string]any{
			"input":      map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": "a cat"}}}}},
			"parameters": map[string]any{"size": "2K", "n": 2, "watermark": false, "thinking_mode": false, "seed": 0},
		}},
		{"gpt_image_2_edit", map[string]any{
			"prompt": "make the background blue", "images": []any{"https://cdn.example/input.png"}, "n": 1, "quality": "high",
		}},
		{"flux_3_image", map[string]any{
			"prompt": "a ceramic cup", "resolution": "768sq", "aspect_ratio": "1:1",
		}},
		{"wan27_i2v", map[string]any{
			"input":      map[string]any{"prompt": "the cat walks", "media": []any{map[string]any{"type": "first_frame", "url": "https://cdn.example/cat.png"}}},
			"parameters": map[string]any{"duration": 5, "resolution": "720P", "watermark": false, "seed": 0},
		}},
		{"hi3d_model_split", map[string]any{
			"mesh_url": "https://cdn.example/model.glb", "split_type": "character", "part": "a", "joint": "ball", "merge": "yes", "format": "glb",
		}},
		{"kirin_tts", map[string]any{
			"text": "你好", "voice_id": "test-voice", "voice_language": "zh", "voice_speed": 1,
		}},
		{"minimax_speech_28_hd", map[string]any{
			"text": "hello", "stream": false, "output_format": "url",
			"voice_setting": map[string]any{"voice_id": "English_Graceful_Lady", "vol": 0, "pitch": 0},
		}},
		{"gemini_3_1_flash_tts_preview", map[string]any{"text": "hello", "voice_name": "Kore"}},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			tc.params["provider_option"] = map[string]any{"enabled": false, "weight": 0}
			body := vtrixRequest(tc.model, tc.params)
			value, err := plugin.Engine.CallPath(t.Context(), "native", []string{"submitGeneration"}, map[string]any{
				"model": tc.model, "body": map[string]any{"kind": "json", "value": body},
			})
			require.NoError(t, err)
			intent := vtrixObject(t, value)
			assert.Equal(t, "submit", intent["kind"])
			assert.Equal(t, tc.model, intent["model"])
			assert.Equal(t, vtrixObject(t, body), intent["requestBody"])

			value, err = plugin.Engine.Call(t.Context(), "buildSubmitRequest", vtrixContext(tc.model, intent["requestBody"].(map[string]any)))
			require.NoError(t, err)
			request := vtrixObject(t, value)
			assert.Equal(t, "POST", request["method"])
			assert.Equal(t, "https://cloud.vtrix.ai/model/v1/generation", request["url"])
			assert.Equal(t, vtrixObject(t, body), request["body"])
			assert.Equal(t, "Bearer test-key", request["headers"].(map[string]any)["Authorization"])
		})
	}
	for _, model := range []string{"wan26_i2i", "wan26_t2i", "vtrix-gpt-4o-mini-transcribe", "vtrix-gpt-4o-transcribe", "vtrix-gpt-4o-transcribe-diarize"} {
		assert.NotContains(t, plugin.Meta.Models, model)
	}
}

func TestVtrixMappedModelsAndQueryIDs(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	body := vtrixRequest("public-alias", map[string]any{
		"input":      map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": "a cat"}}}}},
		"parameters": map[string]any{"n": 1},
	})
	ctx := vtrixContext("public-alias", body)
	ctx["upstreamModel"] = "wan27_image"
	value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
	require.NoError(t, err)
	request := vtrixObject(t, value)
	assert.Equal(t, "wan27_image", request["body"].(map[string]any)["model"])
	assert.Equal(t, "public-alias", body["model"])
	assert.NotContains(t, request, "model")

	value, err = plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId": "task/a?b#c", "model": "wan27_image", "baseUrl": "https://cloud.vtrix.ai", "apiKey": "test-key",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://cloud.vtrix.ai/model/v1/generation/task/task%2Fa%3Fb%23c", vtrixObject(t, value)["url"])
}

func TestVtrixOpenAIImageGeneration(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{
		"model": "wan27_image", "upstreamModel": "wan27_image", "operation": "generate",
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "wan27_image", "prompt": "a cat", "n": 2, "size": "2K", "watermark": false, "seed": 0,
		}},
	})
	require.NoError(t, err)
	decoded := vtrixObject(t, value)
	require.Equal(t, "submit", decoded["kind"])
	require.Equal(t, "wan27_image", decoded["model"])
	body, ok := decoded["requestBody"].(map[string]any)
	require.True(t, ok)
	value, err = plugin.Engine.Call(t.Context(), "buildSubmitRequest", vtrixContext("wan27_image", body))
	require.NoError(t, err)
	request := vtrixObject(t, value)["body"].(map[string]any)
	assert.Equal(t, "wan27_image", request["model"])
	params := request["input"].([]any)[0].(map[string]any)["params"].(map[string]any)
	options := params["parameters"].(map[string]any)
	assert.Equal(t, float64(2), options["n"])
	assert.Equal(t, false, options["watermark"])
	assert.Equal(t, float64(0), options["seed"])
	assert.Equal(t, "2K", options["size"])
}

func TestVtrixUsageUsesFinalRequestQuantities(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	tests := []struct {
		model  string
		params map[string]any
		fact   string
		want   float64
	}{
		{"wan27_image", map[string]any{
			"input":      map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": "a cat"}}}}},
			"parameters": map[string]any{"n": 2},
		}, "image_count", 2},
		{"wan27_i2v", map[string]any{
			"input":      map[string]any{"media": []any{map[string]any{"type": "first_frame", "url": "https://cdn.example/input.png"}}},
			"parameters": map[string]any{"duration": 5, "resolution": "720P"},
		}, "duration", 5},
		{"tencent_hunyuan_3d_pro", map[string]any{"prompt": "a cat", "generate_type": "Normal"}, "asset_count", 1},
		{"minimax_speech_28_hd", map[string]any{"text": " 你好🙂 ", "output_format": "url"}, "characters", 5},
		{"kirin_tts", map[string]any{"text": "你好🙂", "voice_id": "test-voice", "voice_language": "zh"}, "characters", 3},
		{"gemini_3_1_flash_tts_preview", map[string]any{"text": "hello"}, "requests", 1},
		{"kirin_video_to_audio", map[string]any{"video_url": "https://cdn.example/input.mp4"}, "requests", 1},
		{"kirin_text_to_audio", map[string]any{"prompt": "ocean waves", "duration": 4.2}, "duration", 4.2},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "extractUsage", vtrixContext(tc.model, vtrixRequest(tc.model, tc.params)))
			require.NoError(t, err)
			assert.Equal(t, tc.want, vtrixObject(t, value)[tc.fact])
		})
	}
}

func TestVtrixImageUsageProfilesAndBatchModes(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	tests := []struct {
		model  string
		params map[string]any
		facts  map[string]any
	}{
		{"wan25_t2i_preview", map[string]any{"input": map[string]any{"prompt": "a cat"}}, map[string]any{"image_count": float64(4)}},
		{"wan26_image", map[string]any{"parameters": map[string]any{"enable_interleave": true}}, map[string]any{"image_count": float64(5)}},
		{"kirin_v3_omni_image", map[string]any{"result_type": "series"}, map[string]any{"image_count": float64(4)}},
		{"spark_dream_4_0", map[string]any{"sequential_image_generation": "auto"}, map[string]any{"image_count": float64(15)}},
		{"spark_dream_5_0", map[string]any{"sequential_image_generation": "auto", "image": []any{"https://cdn.example/input.png"}}, map[string]any{"image_count": float64(14)}},
		{"spark_dream_5_0_flash", map[string]any{"image": []any{"https://cdn.example/input.png"}, "layer_decomposition": true}, map[string]any{"image_count": float64(17)}},
		{"flux_3_image", map[string]any{"prompt": "a cat", "size": "2048x2048", "resolution": "1k", "aspect_ratio": "4:3"}, map[string]any{"image_count": float64(1), "resolution": "1k"}},
		{"midjourney_V8.2_t2i", map[string]any{"text": "a cat --v 8.1"}, map[string]any{"requests": float64(1)}},
		{"minimax_h3_regeneration", map[string]any{"content": []any{map[string]any{"type": "video_url", "role": "base_video", "video_url": map[string]any{"url": "https://cdn.example/input.mp4"}}}}, map[string]any{"requests": float64(1)}},
		{"veo_3.1_generate_001", map[string]any{"prompt": "a cat", "durationSeconds": 8, "sampleCount": 4}, map[string]any{"requests": float64(4)}},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "extractUsage", vtrixContext(tc.model, vtrixRequest(tc.model, tc.params)))
			require.NoError(t, err)
			assert.Equal(t, tc.facts, vtrixObject(t, value))
		})
	}
}

func TestVtrixCompletionCountsDeliveredImages(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	for _, tc := range []struct {
		name  string
		usage map[string]any
	}{
		{"quantity is not the image count", map[string]any{"quantity": 1}},
		{"reported generated images", map[string]any{"generated_images": 2}},
		{"extra image count", map[string]any{"extra_info": map[string]any{"num_images": 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{
				"output": []any{map[string]any{"content": []any{
					map[string]any{"type": "image", "url": "https://cdn.example/one.png"},
					map[string]any{"type": "image", "url": "https://cdn.example/two.png"},
					map[string]any{"type": "text", "text": "two images"},
				}}},
				"usage": tc.usage,
			}
			value, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", map[string]any{
				"model": "wan27_image", "state": map[string]any{"kind": "image", "usage": map[string]any{"image_count": 4}},
			}, map[string]any{"status": "SUCCESS"}, body)
			require.NoError(t, err)
			assert.Equal(t, map[string]any{"image_count": float64(2)}, vtrixObject(t, value))
		})
	}
	_, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", map[string]any{
		"model": "wan27_image", "state": map[string]any{"kind": "image", "usage": map[string]any{"image_count": 1}},
	}, map[string]any{"status": "SUCCESS"}, map[string]any{
		"usage":  map[string]any{"generated_images": 0},
		"output": []any{map[string]any{"content": []any{map[string]any{"type": "image", "url": "https://cdn.example/one.png"}}}},
	})
	require.Error(t, err)
}

func TestVtrixOpenAIImageMetadataAndMultipart(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	metadata := map[string]any{"params": map[string]any{"parameters": map[string]any{"n": 3, "thinking_mode": false, "seed": 0}}}
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{
		"model": "wan27_image", "operation": "generate",
		"body": map[string]any{"kind": "json", "value": map[string]any{"prompt": "a cat", "metadata": metadata}},
	})
	require.NoError(t, err)
	intent := vtrixObject(t, value)
	require.Equal(t, "submit", intent["kind"])
	params := intent["requestBody"].(map[string]any)["input"].([]any)[0].(map[string]any)["params"].(map[string]any)
	assert.Equal(t, map[string]any{"n": float64(3), "thinking_mode": false, "seed": float64(0)}, params["parameters"])

	for _, tc := range []struct{ model, encoding string }{
		{"kirin_v3_omni_image", "base64"},
		{"gpt_image_2_edit", "dataUrl"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{
				"model": tc.model, "operation": "edit",
				"body": map[string]any{"kind": "multipart", "fields": map[string]any{"prompt": []string{"make it blue"}},
					"files": []any{map[string]any{"field": "image", "ref": "request_file:image#1", "mimeType": "image/png"}}},
			})
			require.NoError(t, err)
			intent := vtrixObject(t, value)
			require.Equal(t, "submit", intent["kind"])
			params := intent["requestBody"].(map[string]any)["input"].([]any)[0].(map[string]any)["params"].(map[string]any)
			var reference any
			if tc.encoding == "base64" {
				reference = params["image_list"].([]any)[0].(map[string]any)["image"]
			} else {
				reference = params["images"].([]any)[0]
			}
			assert.Equal(t, map[string]any{"__fileRef": "request_file:image#1", "encoding": tc.encoding, "maxBytes": float64(10485760)}, reference)
		})
	}
}

func TestVtrixRejectsUnsafeNativeQuantities(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	tests := []struct {
		name string
		body map[string]any
	}{
		{"empty batch", map[string]any{"model": "wan27_image", "input": []any{}}},
		{"multiple inputs", map[string]any{"model": "wan27_image", "input": []any{map[string]any{"params": map[string]any{}}, map[string]any{"params": map[string]any{}}}}},
		{"zero count", vtrixRequest("gpt_image_2", map[string]any{"prompt": "a cat", "n": 0})},
		{"negative count", vtrixRequest("gpt_image_2", map[string]any{"prompt": "a cat", "n": -1})},
		{"fractional count", vtrixRequest("gpt_image_2", map[string]any{"prompt": "a cat", "n": 1.5})},
		{"oversized count", vtrixRequest("gpt_image_2", map[string]any{"prompt": "a cat", "n": 129})},
		{"artifact limit", vtrixRequest("wan27_image", map[string]any{"parameters": map[string]any{"n": 65}})},
		{"nested oversized count", vtrixRequest("wan27_image", map[string]any{
			"input":      map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": "a cat"}}}}},
			"parameters": map[string]any{"n": 129},
		})},
		{"negative duration", vtrixRequest("kirin_text_to_audio", map[string]any{"prompt": "ocean waves", "duration": -1})},
		{"zero duration", vtrixRequest("kirin_text_to_audio", map[string]any{"prompt": "ocean waves", "duration": 0})},
		{"oversized duration", vtrixRequest("kirin_text_to_audio", map[string]any{"prompt": "ocean waves", "duration": 3601})},
		{"minimax text limit", vtrixRequest("minimax_speech_28_hd", map[string]any{"text": strings.Repeat("a", 10000)})},
		{"kirin text limit", vtrixRequest("kirin_tts", map[string]any{"text": strings.Repeat("a", 1001), "voice_id": "test-voice", "voice_language": "zh"})},
		{"midjourney repeat", vtrixRequest("midjourney_diffusion", map[string]any{"text": "a cat --repeat 40"})},
		{"midjourney permutation", vtrixRequest("midjourney_diffusion", map[string]any{"text": "a {red,blue} cat"})},
		{"unsupported video batch", vtrixRequest("sora_2_t2v", map[string]any{"prompt": "a cat", "n": 2})},
		{"oversized video sample count", vtrixRequest("veo_3.1_generate_001", map[string]any{"sampleCount": 5})},
		{"regeneration cannot override duration", vtrixRequest("minimax_h3_regeneration", map[string]any{"duration": 5})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := plugin.Engine.CallPath(t.Context(), "native", []string{"submitGeneration"}, map[string]any{
				"model": tc.body["model"], "body": map[string]any{"kind": "json", "value": tc.body},
			})
			require.Error(t, err)
		})
	}
}

func TestVtrixSubmissionWaitsForRealOutput(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	ctx := vtrixContext("kirin_tts", vtrixRequest("kirin_tts", map[string]any{"text": "hello", "voice_id": "test-voice", "voice_language": "en"}))
	for _, status := range []string{"created", "in_progress", "completed"} {
		t.Run(status+" without output", func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", ctx, map[string]any{
				"statusCode": 200, "body": map[string]any{"id": "upstream-task", "status": status, "model": "kirin_tts"},
			})
			require.NoError(t, err)
			result := vtrixObject(t, value)
			assert.Equal(t, "upstream-task", result["taskId"])
			assert.Nil(t, result["immediate"])
		})
	}

	completed := map[string]any{
		"id": "upstream-task", "status": "completed", "model": "kirin_tts",
		"output": []any{map[string]any{"content": []any{map[string]any{"type": "audio", "url": "https://cdn.example/audio.mp3"}}}},
	}
	value, err := plugin.Engine.Call(t.Context(), "parseSubmitResponse", ctx, map[string]any{"statusCode": 200, "body": completed})
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", vtrixObject(t, value)["immediate"].(map[string]any)["status"])
}

func TestVtrixPollingRequiresExpectedDeliverable(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	tests := []struct {
		name, model, status, contentType, url, want string
	}{
		{"image", "wan27_image", "completed", "image", "https://cdn.example/image.png", "SUCCESS"},
		{"video", "wan27_i2v", "completed", "video", "https://cdn.example/video.mp4", "SUCCESS"},
		{"audio", "kirin_tts", "completed", "audio", "https://cdn.example/audio.mp3", "SUCCESS"},
		{"3D model", "tencent_hunyuan_3d_pro", "completed", "glb", "https://cdn.example/model.glb", "SUCCESS"},
		{"HI3D model", "hi3d_image_to_3d", "completed", "model3d", "https://cdn.example/model.stl", "SUCCESS"},
		{"relief image", "hi3d_image_to_relief", "completed", "image", "https://cdn.example/relief.png", "SUCCESS"},
		{"wrong artifact type", "kirin_tts", "completed", "image", "https://cdn.example/image.png", "FAILURE"},
		{"missing audio URL", "kirin_tts", "completed", "audio", "", "FAILURE"},
		{"empty terminal output", "wan27_image", "completed", "", "", "FAILURE"},
		{"unknown status", "wan27_image", "provider_unknown", "", "", "UNKNOWN"},
		{"not a downloadable image", "wan27_image", "completed", "image", "not-an-image", "FAILURE"},
		{"not a downloadable audio file", "kirin_tts", "completed", "audio", "not-an-audio-file", "FAILURE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"status": tc.status, "output": []any{map[string]any{"content": []any{}}}}
			if tc.contentType != "" {
				body["output"] = []any{map[string]any{"content": []any{map[string]any{"type": tc.contentType, "url": tc.url}}}}
			}
			value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{"model": tc.model, "upstreamModel": tc.model}, body)
			require.NoError(t, err)
			assert.Equal(t, tc.want, vtrixObject(t, value)["status"])
		})
	}
	for _, url := range []any{true, 42, map[string]any{"url": "https://cdn.example/image.png"}} {
		value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{"model": "wan27_image"}, map[string]any{
			"status": "completed", "output": []any{map[string]any{"content": []any{map[string]any{"type": "image", "url": url}}}},
		})
		require.NoError(t, err)
		assert.Equal(t, "FAILURE", vtrixObject(t, value)["status"])
	}
	for _, status := range []string{"created", "queued", "in_queue"} {
		t.Run(status, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{"model": "kirin_tts"}, map[string]any{"status": status})
			require.NoError(t, err)
			assert.Contains(t, []string{"IN_PROGRESS", "SUBMITTED", "NOT_START"}, vtrixObject(t, value)["status"])
		})
	}
}

func TestVtrixAudioAndModelArtifacts(t *testing.T) {
	plugin := loadVtrixPlugin(t)
	for _, tc := range []struct{ model, contentType, artifactType, url, format string }{
		{"kirin_tts", "audio", "audio", "https://cdn.example/audio.mp3", ""},
		{"tencent_hunyuan_3d_pro", "glb", "file", "https://cdn.example/model.zip", "obj"},
		{"hi3d_image_to_3d", "model3d", "file", "https://cdn.example/model.stl", "stl"},
		{"hi3d_image_to_relief", "model3d", "image", "https://cdn.example/relief.png", "png"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			data := map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"type": tc.contentType, "url": tc.url, "format": tc.format}}}}}
			value, err := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{"status": "SUCCESS", "model": tc.model, "data": data})
			require.NoError(t, err)
			encoded, err := common.Marshal(value)
			require.NoError(t, err)
			var artifacts []map[string]any
			require.NoError(t, common.Unmarshal(encoded, &artifacts))
			require.Len(t, artifacts, 1)
			assert.Equal(t, tc.artifactType, artifacts[0]["type"])

			value, err = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
				"model": tc.model, "data": data, "artifactKey": artifacts[0]["key"], "clientRequest": map[string]any{"method": "HEAD"},
			})
			require.NoError(t, err)
			request := vtrixObject(t, value)
			assert.Equal(t, tc.url, request["url"])
			assert.Equal(t, "HEAD", request["method"])
			assert.Equal(t, true, request["credentialless"])
		})
	}
}
