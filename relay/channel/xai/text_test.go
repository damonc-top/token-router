package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unexpectedEOFReadCloser struct {
	payload []byte
	sent    bool
}

func (r *unexpectedEOFReadCloser) Read(p []byte) (int, error) {
	if r.sent {
		return 0, io.ErrUnexpectedEOF
	}
	r.sent = true
	return copy(p, r.payload), nil
}

func (r *unexpectedEOFReadCloser) Close() error {
	return nil
}

func newXAIStreamTestContext(t *testing.T, body io.ReadCloser) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-4.5",
		},
	}
	return c, recorder, resp, info
}

func xAIStreamChunk() string {
	return `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"grok-4.5","choices":[{"index":0,"delta":{"content":"hello"}}],"usage":{"prompt_tokens":1,"total_tokens":2}}` + "\n"
}

func TestXAIStreamHandlerReemitsDoneAfterUpstreamDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := xAIStreamChunk() + "data: [DONE]\n"
	c, recorder, resp, info := newXAIStreamTestContext(t, io.NopCloser(strings.NewReader(body)))

	usage, err := xAIStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, 2, usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), `"content":"hello"`)
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestXAIStreamHandlerDoesNotFabricateDoneAfterUnexpectedEOF(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	c, recorder, resp, info := newXAIStreamTestContext(t, &unexpectedEOFReadCloser{
		payload: []byte(xAIStreamChunk()),
	})

	usage, err := xAIStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
	assert.Contains(t, recorder.Body.String(), `"content":"hello"`)
	assert.NotContains(t, recorder.Body.String(), "data: [DONE]")
}
