package messagelog

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/message_log_setting"
)

const maxMessageLogSizeCleanupRows = 5000

// If WAL (+ main) is still this large after passive checkpoint, try TRUNCATE.
const messageLogWALTruncateThresholdBytes = 64 * 1024 * 1024

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
				// Still checkpoint when disabled so a leftover WAL can shrink.
				checkpointMessageLogDB()
				return
			}
			retentionDeleted := cleanupByRetention(setting.RetentionDays)
			sizeDeleted := cleanupByDiskSize(setting.MaxSizeMB)
			if retentionDeleted > 0 || sizeDeleted > 0 {
				checkpointMessageLogDB()
				// Prefer reclaiming WAL over full-file VACUUM. VACUUM is throttled.
				if model.GetMessageLogTableSizeMB() > int64(setting.MaxSizeMB) {
					if err := model.MaybeVacuumMessageLogDB(); err != nil {
						common.SysError("message_log: VACUUM failed: " + err.Error())
					}
				}
			} else {
				checkpointMessageLogDB()
			}
		}()
	}
}

func checkpointMessageLogDB() {
	if !common.UsingLogDatabase(common.DatabaseTypeSQLite) {
		return
	}
	if err := model.CheckpointMessageLogDB("PASSIVE"); err != nil {
		common.SysLog("message_log: passive wal_checkpoint: " + err.Error())
	}
	usage, err := model.GetMessageLogDiskUsageBytes()
	if err != nil {
		return
	}
	if usage < messageLogWALTruncateThresholdBytes {
		return
	}
	if err := model.CheckpointMessageLogDB("TRUNCATE"); err != nil {
		common.SysLog("message_log: truncate wal_checkpoint: " + err.Error())
		return
	}
	if after, err := model.GetMessageLogDiskUsageBytes(); err == nil {
		common.SysLog(fmt.Sprintf("message_log: wal checkpoint truncate disk usage now %d MiB", after/(1024*1024)))
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
	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024

	// Prefer on-disk usage (includes WAL). Fall back to payload SUM.
	currentSizeBytes, err := model.GetMessageLogDiskUsageBytes()
	if err != nil || currentSizeBytes <= 0 {
		currentSizeBytes, err = model.GetMessageLogBodySizeBytes()
		if err != nil {
			common.SysError("message_log: read body size failed: " + err.Error())
			return 0
		}
	}
	if currentSizeBytes <= maxSizeBytes {
		return 0
	}

	// Body-size based deletion still drives which rows to remove.
	bodySizeBytes, bodyErr := model.GetMessageLogBodySizeBytes()
	targetBytes := currentSizeBytes - maxSizeBytes
	if bodyErr == nil && bodySizeBytes > maxSizeBytes {
		// Also ensure payload itself is under the cap.
		if bodySizeBytes-maxSizeBytes > targetBytes {
			targetBytes = bodySizeBytes - maxSizeBytes
		}
	}

	deleted, err := model.DeleteOldestMessageLogsBySize(
		targetBytes,
		maxMessageLogSizeCleanupRows,
	)
	if err != nil {
		common.SysError("message_log: cleanup by disk size failed: " + err.Error())
		return 0
	}
	if deleted > 0 {
		common.SysLog(fmt.Sprintf("message_log: cleaned up %d oldest records to enforce the %d MiB disk/payload limit", deleted, maxSizeMB))
	}
	return deleted
}