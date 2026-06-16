package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeContentTypePrefersJSON(t *testing.T) {
	require.Equal(t, "application/json", normalizeContentType([]string{"text/plain; charset=utf-8, application/json"}))
}

func TestNormalizeContentTypePrefersEventStreamAfterJSON(t *testing.T) {
	require.Equal(t, "text/event-stream; charset=utf-8", normalizeContentType([]string{"text/plain", "text/event-stream; charset=utf-8"}))
}

func TestNormalizeContentTypeKeepsSingleValue(t *testing.T) {
	require.Equal(t, "text/plain; charset=utf-8", normalizeContentType([]string{"text/plain; charset=utf-8"}))
}
