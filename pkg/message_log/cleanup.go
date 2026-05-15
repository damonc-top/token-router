package messagelog

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/message_log_setting"
)

func startCleanupLoop() {
	if !common.IsMasterNode {
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		setting := message_log_setting.GetSetting()
		if !setting.Enabled {
			continue
		}
		cleanupByRetention(setting.RetentionDays)
		cleanupByDiskSize(setting.MaxSizeMB)
	}
}

func cleanupByRetention(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	deleted, err := model.DeleteMessageLogsBefore(cutoff)
	if err != nil {
		common.SysError("message_log: cleanup by retention failed: " + err.Error())
		return
	}
	if deleted > 0 {
		common.SysLog(fmt.Sprintf("message_log: cleaned up %d expired records", deleted))
	}
}

func cleanupByDiskSize(maxSizeMB int) {
	if maxSizeMB <= 0 {
		return
	}
	currentSizeMB := model.GetMessageLogTableSizeMB()
	if currentSizeMB <= int64(maxSizeMB) {
		return
	}
	for currentSizeMB > int64(maxSizeMB) {
		deleted, err := model.DeleteOldestMessageLogs(200)
		if err != nil {
			common.SysError("message_log: cleanup by disk size failed: " + err.Error())
			return
		}
		if deleted == 0 {
			break
		}
		currentSizeMB = model.GetMessageLogTableSizeMB()
	}
}
