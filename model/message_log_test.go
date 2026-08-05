package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
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

func TestDeleteOldestMessageLogsBySizeDeletesEnoughOldestRecords(t *testing.T) {
	withMessageLogTestDB(t)
	for _, record := range []struct {
		id       int
		bodySize int64
	}{
		{id: 1, bodySize: 100},
		{id: 2, bodySize: 200},
		{id: 3, bodySize: 300},
	} {
		require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
			Id:        record.id,
			RequestId: "req",
			BodySize:  record.bodySize,
			CreatedAt: int64(record.id),
		}).Error)
	}

	deleted, err := DeleteOldestMessageLogsBySize(250, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)
	var ids []int
	require.NoError(t, MSG_LOG_DB.Model(&MessageLog{}).Order("id ASC").Pluck("id", &ids).Error)
	assert.Equal(t, []int{3}, ids)
}

func TestDeleteOldestMessageLogsBySizeHonorsMaxRows(t *testing.T) {
	withMessageLogTestDB(t)
	for i := 1; i <= 3; i++ {
		require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
			Id:        i,
			RequestId: "req",
			BodySize:  100,
			CreatedAt: int64(i),
		}).Error)
	}

	deleted, err := DeleteOldestMessageLogsBySize(1000, 2)

	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)
	var ids []int
	require.NoError(t, MSG_LOG_DB.Model(&MessageLog{}).Order("id ASC").Pluck("id", &ids).Error)
	assert.Equal(t, []int{3}, ids)
}

func TestGetMessageLogBodySizeBytes(t *testing.T) {
	withMessageLogTestDB(t)
	require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
		Id:        1,
		RequestId: "req-1",
		BodySize:  100,
		CreatedAt: 1,
	}).Error)
	require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
		Id:        2,
		RequestId: "req-2",
		BodySize:  200,
		CreatedAt: 2,
	}).Error)

	sizeBytes, err := GetMessageLogBodySizeBytes()

	require.NoError(t, err)
	assert.EqualValues(t, 300, sizeBytes)
}

func TestMessageLogStoresStreamDiagnostics(t *testing.T) {
	withMessageLogTestDB(t)
	require.NoError(t, MSG_LOG_DB.Create(&MessageLog{
		Id:                  1,
		RequestId:           "req",
		IsStream:            true,
		StreamEndReason:     "scanner_error",
		StreamResponseCount: 3,
		CreatedAt:           1,
	}).Error)

	var stored MessageLog
	require.NoError(t, MSG_LOG_DB.First(&stored, 1).Error)

	assert.True(t, stored.IsStream)
	assert.Equal(t, "scanner_error", stored.StreamEndReason)
	assert.Equal(t, 3, stored.StreamResponseCount)
}
