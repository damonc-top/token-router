package common

import (
	"path/filepath"
	"runtime"
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

func TestMsgLogSQLiteDSNUsesDedicatedPragmas(t *testing.T) {
	assert.Equal(
		t,
		"message-log.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)",
		msgLogSQLiteDSN(MsgLogSQLiteFileName),
	)
	assert.NotEqual(t, sqliteDSN(MsgLogSQLiteFileName), msgLogSQLiteDSN(MsgLogSQLiteFileName))

	db, err := gorm.Open(sqlite.Open(msgLogSQLiteDSN(filepath.Join(t.TempDir(), "msg.db"))), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
	})

	var journalMode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	assert.Equal(t, "wal", strings.ToLower(journalMode))

	var synchronous int64
	require.NoError(t, db.Raw("PRAGMA synchronous").Scan(&synchronous).Error)
	assert.EqualValues(t, 1, synchronous) // NORMAL
}

func TestNormalizeDiskCachePathRejectsForeignOSAbsolute(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := NormalizeDiskCachePath("/Users/mac/temp/cache/tokenrouter")
		assert.NotEqual(t, "/Users/mac/temp/cache/tokenrouter", got)
		assert.NotEmpty(t, got)
	} else {
		got := NormalizeDiskCachePath(`C:\Users\mac\temp\cache`)
		assert.NotEqual(t, `C:\Users\mac\temp\cache`, got)
		assert.NotEmpty(t, got)
	}
}

func TestNormalizeDiskCachePathEmptyUsesTemp(t *testing.T) {
	got := NormalizeDiskCachePath("   ")
	assert.NotEmpty(t, got)
}

func TestSetDiskCacheConfigNormalizesPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path sample")
	}
	SetDiskCacheConfig(DiskCacheConfig{
		Enabled: true,
		Path:    "/Users/mac/temp/cache/tokenrouter",
	})
	assert.NotEqual(t, "/Users/mac/temp/cache/tokenrouter", GetDiskCachePath())
}