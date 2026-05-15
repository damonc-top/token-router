package model

import (
	"github.com/QuantumNous/new-api/common"
)

type MessageLog struct {
	Id                int    `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestId         string `json:"request_id" gorm:"type:varchar(64);index:idx_msglog_request_id;default:''"`
	UserId            int    `json:"user_id" gorm:"index"`
	TokenId           int    `json:"token_id" gorm:"index"`
	ChannelId         int    `json:"channel_id" gorm:"index"`
	ModelName         string `json:"model_name" gorm:"type:varchar(128);index"`
	UpstreamModelName string `json:"upstream_model_name" gorm:"type:varchar(128)"`
	GroupName         string `json:"group_name" gorm:"type:varchar(64);index"`
	RequestURL        string `json:"request_url" gorm:"type:text"`
	RequestMethod     string `json:"request_method" gorm:"type:varchar(10)"`
	RequestHeaders    string `json:"request_headers" gorm:"type:text"`
	RequestBody       []byte `json:"request_body" gorm:"type:longblob"`
	ResponseStatus    int    `json:"response_status"`
	ResponseHeaders   string `json:"response_headers" gorm:"type:text"`
	ResponseBody      []byte `json:"response_body" gorm:"type:longblob"`
	IsStream          bool   `json:"is_stream"`
	BodySize          int64  `json:"body_size" gorm:"index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_msglog_created_at"`
}

func (MessageLog) TableName() string {
	return "message_logs"
}

type MessageLogStats struct {
	Count  int64 `json:"count"`
	SizeMB int64 `json:"size_mb"`
}

func CreateMessageLog(log *MessageLog) error {
	return MSG_LOG_DB.Create(log).Error
}

func BatchCreateMessageLogs(logs []*MessageLog) error {
	if len(logs) == 0 {
		return nil
	}
	return MSG_LOG_DB.CreateInBatches(logs, 50).Error
}

func GetMessageLogs(page, pageSize int, requestId string, modelName string, channelId int, userId int) ([]*MessageLog, int64, error) {
	var logs []*MessageLog
	var total int64

	tx := MSG_LOG_DB.Model(&MessageLog{})
	if requestId != "" {
		tx = tx.Where("request_id = ?", requestId)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if channelId > 0 {
		tx = tx.Where("channel_id = ?", channelId)
	}
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}

	err := tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = tx.Select("id, request_id, user_id, token_id, channel_id, model_name, upstream_model_name, group_name, request_url, request_method, response_status, is_stream, body_size, created_at").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}

func GetMessageLogById(id int) (*MessageLog, error) {
	var log MessageLog
	err := MSG_LOG_DB.First(&log, id).Error
	return &log, err
}

func DeleteMessageLogsBefore(timestamp int64) (int64, error) {
	var total int64
	batchSize := 500
	for {
		result := MSG_LOG_DB.Where("created_at < ?", timestamp).Limit(batchSize).Delete(&MessageLog{})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < int64(batchSize) {
			break
		}
	}
	return total, nil
}

func DeleteOldestMessageLogs(limit int) (int64, error) {
	result := MSG_LOG_DB.Order("created_at ASC").Limit(limit).Delete(&MessageLog{})
	return result.RowsAffected, result.Error
}

func GetMessageLogStats() (*MessageLogStats, error) {
	var stats MessageLogStats
	err := MSG_LOG_DB.Model(&MessageLog{}).Count(&stats.Count).Error
	if err != nil {
		return nil, err
	}
	stats.SizeMB = getMessageLogTableSizeMB()
	return &stats, nil
}

func getMessageLogTableSizeMB() int64 {
	var sizeMB int64
	if common.UsingPostgreSQL {
		MSG_LOG_DB.Raw("SELECT COALESCE(pg_total_relation_size('message_logs'), 0) / (1024*1024)").Scan(&sizeMB)
	} else if common.UsingMySQL {
		MSG_LOG_DB.Raw("SELECT COALESCE((DATA_LENGTH + INDEX_LENGTH), 0) / (1024*1024) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'message_logs'").Scan(&sizeMB)
	} else {
		var totalBytes int64
		MSG_LOG_DB.Model(&MessageLog{}).Select("COALESCE(SUM(body_size), 0)").Scan(&totalBytes)
		sizeMB = totalBytes / (1024 * 1024)
	}
	return sizeMB
}

func GetMessageLogTableSizeMB() int64 {
	return getMessageLogTableSizeMB()
}

func GetMessageLogCount() int64 {
	var count int64
	MSG_LOG_DB.Model(&MessageLog{}).Count(&count)
	return count
}

func DeleteAllMessageLogs() (int64, error) {
	result := MSG_LOG_DB.Where("1 = 1").Delete(&MessageLog{})
	return result.RowsAffected, result.Error
}

