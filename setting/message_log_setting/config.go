package message_log_setting

import "github.com/QuantumNous/new-api/setting/config"

type MessageLogSetting struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days"`
	MaxSizeMB     int  `json:"max_size_mb"`
}

var messageLogSetting = MessageLogSetting{
	Enabled:       false,
	RetentionDays: 7,
	MaxSizeMB:     1024,
}

func init() {
	config.GlobalConfig.Register("message_log_setting", &messageLogSetting)
}

func GetSetting() MessageLogSetting {
	s := messageLogSetting
	if s.RetentionDays > 30 {
		s.RetentionDays = 30
	}
	if s.RetentionDays < 1 {
		s.RetentionDays = 1
	}
	if s.MaxSizeMB > 102400 {
		s.MaxSizeMB = 102400
	}
	if s.MaxSizeMB < 1 {
		s.MaxSizeMB = 1
	}
	return s
}

func IsEnabled() bool {
	return messageLogSetting.Enabled
}
