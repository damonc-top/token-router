package common

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSQLiteDSNAppliesRequiredPragmas(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(sqliteDSN(filepath.Join(t.TempDir(), "test.db"))), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})

	var journalMode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)

	var busyTimeout int64
	require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)

	assert.Equal(t, "wal", strings.ToLower(journalMode))
	assert.EqualValues(t, 30000, busyTimeout)
}

func TestSQLiteDSNAppendsPragmasToExistingQuery(t *testing.T) {
	assert.Equal(
		t,
		"file:test.db?cache=shared&_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)",
		sqliteDSN("file:test.db?cache=shared"),
	)
}
