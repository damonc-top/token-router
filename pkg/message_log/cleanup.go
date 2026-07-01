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
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysError(fmt.Sprintf("message_log: cleanup panic recovered: %v", r))
				}
			}()
			setting := message_log_setting.GetSetting()
			if !setting.Enabled {
				return
			}
			deleted := cleanupByRetention(setting.RetentionDays)
			deleted += cleanupByDiskSize(setting.MaxSizeMB)
			if deleted > 0 {
				vacuumIfSQLite()
			}
		}()
	}
}

func vacuumIfSQLite() {
	if !common.UsingLogDatabase(common.DatabaseTypeSQLite) {
		return
	}
	// We can't run VACUUM through GORM with prepared statements enabled,
	// so force a raw tx once per cleanup cycle.
	err := model.VacuumMessageLogDB()
	if err != nil {
		common.SysError("message_log: VACUUM failed: " + err.Error())
	}
}

func cleanupByRetention(retentionDays int) int64 {
	if retentionDays <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	deleted, err := model.DeleteMessageLogsBefore(cutoff)
	if err != nil {
		common.SysError("message_log: cleanup by retention failed: " + err.Error())
		return 0
	}
	if deleted > 0 {
		common.SysLog(fmt.Sprintf("message_log: cleaned up %d expired records", deleted))
	}
	return deleted
}

func cleanupByDiskSize(maxSizeMB int) int64 {
	if maxSizeMB <= 0 {
		return 0
	}
	currentSizeMB := model.GetMessageLogTableSizeMB()
	if currentSizeMB <= int64(maxSizeMB) {
		return 0
	}
	var totalDeleted int64
	for currentSizeMB > int64(maxSizeMB) {
		deleted, err := model.DeleteOldestMessageLogs(200)
		if err != nil {
			common.SysError("message_log: cleanup by disk size failed: " + err.Error())
			return totalDeleted
		}
		if deleted == 0 {
			break
		}
		totalDeleted += deleted
		currentSizeMB = model.GetMessageLogTableSizeMB()
	}
	return totalDeleted
}
