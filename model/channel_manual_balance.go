package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	ChannelManualBalanceResetDaily     = "daily"
	ChannelManualBalanceResetWeekly    = "weekly"
	ChannelManualBalanceResetMonthly   = "monthly"
	ChannelManualBalanceResetQuarterly = "quarterly"
	ChannelWeightRandomMinBalanceUSD   = 5.0
)

func NormalizeChannelManualBalanceResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case ChannelManualBalanceResetDaily,
		ChannelManualBalanceResetWeekly,
		ChannelManualBalanceResetMonthly,
		ChannelManualBalanceResetQuarterly:
		return strings.TrimSpace(period)
	default:
		return ChannelManualBalanceResetMonthly
	}
}

func CalcNextChannelManualBalanceResetTime(base time.Time, period string) int64 {
	period = NormalizeChannelManualBalanceResetPeriod(period)
	var next time.Time
	switch period {
	case ChannelManualBalanceResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case ChannelManualBalanceResetWeekly:
		weekday := int(base.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 8-weekday)
	case ChannelManualBalanceResetMonthly:
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case ChannelManualBalanceResetQuarterly:
		month := int(base.Month())
		nextQuarterMonth := time.Month(((month-1)/3+1)*3 + 1)
		year := base.Year()
		if nextQuarterMonth > 12 {
			nextQuarterMonth -= 12
			year++
		}
		next = time.Date(year, nextQuarterMonth, 1, 0, 0, 0, 0, base.Location())
	default:
		return 0
	}
	return next.Unix()
}

func ApplyChannelManualBalanceDefaults(channel *Channel) error {
	if channel == nil {
		return errors.New("channel is nil")
	}
	if channel.ManualBalanceAmount < 0 {
		return errors.New("manual balance amount must be greater than or equal to 0")
	}
	channel.ManualBalanceResetPeriod = NormalizeChannelManualBalanceResetPeriod(channel.ManualBalanceResetPeriod)
	if !channel.ManualBalanceEnabled {
		channel.ManualBalanceNextResetTime = 0
		return nil
	}
	now := time.Now()
	channel.Balance = channel.ManualBalanceAmount
	channel.BalanceUpdatedTime = now.Unix()
	channel.ManualBalanceNextResetTime = CalcNextChannelManualBalanceResetTime(now, channel.ManualBalanceResetPeriod)
	return nil
}

func UpdateChannelManualBalanceConfig(id int, enabled bool, amount float64, period string) error {
	if id <= 0 {
		return errors.New("invalid channel id")
	}
	if amount < 0 {
		return errors.New("manual balance amount must be greater than or equal to 0")
	}
	now := time.Now()
	period = NormalizeChannelManualBalanceResetPeriod(period)
	updates := map[string]interface{}{
		"manual_balance_enabled":         enabled,
		"manual_balance_amount":          amount,
		"manual_balance_reset_period":    period,
		"manual_balance_next_reset_time": int64(0),
	}
	if enabled {
		updates["balance"] = amount
		updates["balance_updated_time"] = now.Unix()
		updates["manual_balance_next_reset_time"] = CalcNextChannelManualBalanceResetTime(now, period)
	}
	return DB.Model(&Channel{}).Where("id = ?", id).Updates(updates).Error
}

func updateChannelManualBalanceUsage(id int, quota int) {
	if id <= 0 || quota == 0 || common.QuotaPerUnit <= 0 {
		return
	}
	delta := float64(quota) / common.QuotaPerUnit
	var balanceExpr interface{}
	if delta > 0 {
		balanceExpr = gorm.Expr("CASE WHEN balance - ? < 0 THEN 0 ELSE balance - ? END", delta, delta)
	} else {
		balanceExpr = gorm.Expr("balance - ?", delta)
	}
	err := DB.Model(&Channel{}).
		Where("id = ? AND manual_balance_enabled = ?", id, true).
		Updates(map[string]interface{}{
			"balance":              balanceExpr,
			"balance_updated_time": common.GetTimestamp(),
		}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to update channel manual balance: channel_id=%d, delta_quota=%d, error=%v", id, quota, err))
	}
}

func ResetDueChannelManualBalances(batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 300
	}
	now := common.GetTimestamp()
	var channels []Channel
	if err := DB.Where("manual_balance_enabled = ? AND manual_balance_next_reset_time > 0 AND manual_balance_next_reset_time <= ?", true, now).
		Order("manual_balance_next_reset_time asc").
		Limit(batchSize).
		Find(&channels).Error; err != nil {
		return 0, err
	}
	resetCount := 0
	for _, channel := range channels {
		dueAt := channel.ManualBalanceNextResetTime
		next := CalcNextChannelManualBalanceResetTime(time.Unix(dueAt, 0), channel.ManualBalanceResetPeriod)
		for next > 0 && next <= now {
			next = CalcNextChannelManualBalanceResetTime(time.Unix(next, 0), channel.ManualBalanceResetPeriod)
		}
		result := DB.Model(&Channel{}).
			Where("id = ? AND manual_balance_enabled = ? AND manual_balance_next_reset_time = ?", channel.Id, true, dueAt).
			Updates(map[string]interface{}{
				"balance":                        channel.ManualBalanceAmount,
				"balance_updated_time":           now,
				"manual_balance_next_reset_time": next,
			})
		if result.Error != nil {
			return resetCount, result.Error
		}
		resetCount += int(result.RowsAffected)
	}
	return resetCount, nil
}

func (channel *Channel) CanParticipateInWeightRandom() bool {
	if channel == nil {
		return false
	}
	if !channel.ManualBalanceEnabled {
		return true
	}
	return channel.Balance >= ChannelWeightRandomMinBalanceUSD
}
