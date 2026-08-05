package messagelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntryToModelPreservesStreamDiagnostics(t *testing.T) {
	record := entryToModel(&LogEntry{
		RequestId:           "req",
		IsStream:            true,
		StreamEndReason:     "scanner_error",
		StreamResponseCount: 3,
	})

	require.NotNil(t, record)
	assert.True(t, record.IsStream)
	assert.Equal(t, "scanner_error", record.StreamEndReason)
	assert.Equal(t, 3, record.StreamResponseCount)
}
