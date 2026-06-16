package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

// normalizeContentType collapses a possibly multi-valued upstream Content-Type
// into a single sane value. Some upstream proxies (e.g. misconfigured LiteLLM or
// intermediate CDNs) may return multiple Content-Type values, such as
// "text/plain; charset=utf-8, application/json". If the first value (text/plain)
// is forwarded as-is, downstream SDKs (e.g. Claude Code) expecting JSON will
// parse the body as a string, causing TypeError on field access like x.usage.input_tokens.
// This function picks the most structured type when multiple values are present.
func normalizeContentType(values []string) string {
	if len(values) == 0 {
		return ""
	}
	// Split on comma to handle both multi-header and single-header comma-separated forms.
	flatten := make([]string, 0, len(values))
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			if t := strings.TrimSpace(p); t != "" {
				flatten = append(flatten, t)
			}
		}
	}
	if len(flatten) == 1 {
		return flatten[0]
	}
	for _, v := range flatten {
		if strings.Contains(strings.ToLower(v), "application/json") {
			return v
		}
	}
	for _, v := range flatten {
		if strings.Contains(strings.ToLower(v), "text/event-stream") {
			return v
		}
	}
	return values[0]
}

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. It returns false for Content-Length
// (managed separately) and X-Oneapi-Request-Id (to preserve the local instance
// ID). When the upstream header is X-Oneapi-Request-Id, the value is captured
// into the Gin context for later logging.
func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	return true
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		for k, v := range src.Header {
			if !ShouldCopyUpstreamHeader(c, k, v) {
				continue
			}
			if strings.EqualFold(k, "Content-Type") {
				c.Writer.Header().Set(k, normalizeContentType(v))
				continue
			}
			c.Writer.Header().Set(k, v[0])
		}
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
