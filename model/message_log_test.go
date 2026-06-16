package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withMessageLogTestDB(t *testing.T) {
	t.Helper()
	original := MSG_LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MessageLog{}))
	MSG_LOG_DB = db
	t.Cleanup(func() {
		MSG_LOG_DB = original
	})
}

func seedMessageLog(t *testing.T, id int, createdAt int64) {
	t.Helper()
	require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
		Id:        id,
		RequestId: "req",
		CreatedAt: createdAt,
	}).Error)
}

func TestDeleteMessageLogsBeforeDeletesInBatches(t *testing.T) {
	withMessageLogTestDB(t)
	for i := 1; i <= 1205; i++ {
		seedMessageLog(t, i, int64(i))
	}

	deleted, err := DeleteMessageLogsBefore(1001)

	require.NoError(t, err)
	require.EqualValues(t, 1000, deleted)
	require.EqualValues(t, 205, GetMessageLogCount())
}

func TestDeleteOldestMessageLogsHonorsLimit(t *testing.T) {
	withMessageLogTestDB(t)
	for i := 1; i <= 5; i++ {
		seedMessageLog(t, i, int64(100+i))
	}

	deleted, err := DeleteOldestMessageLogs(2)

	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	var ids []int
	require.NoError(t, MSG_LOG_DB.Model(&MessageLog{}).Order("id ASC").Pluck("id", &ids).Error)
	require.Equal(t, []int{3, 4, 5}, ids)
}
