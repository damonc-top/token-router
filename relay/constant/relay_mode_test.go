package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPath2RelayModeClaudeCountTokens(t *testing.T) {
	require.Equal(t, RelayModeClaudeCountTokens, Path2RelayMode("/v1/messages/count_tokens"))
	require.Equal(t, RelayModeClaudeCountTokens, Path2RelayMode("/v1/messages/count_tokens/"))
}

func TestPath2RelayModeAlphaSearch(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/alpha/search", want: RelayModeAlphaSearch},
		{path: "/v1/alpha/search?foo=1", want: RelayModeAlphaSearch},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}
