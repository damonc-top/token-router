package model

import (
	"testing"
	"time"
)

func TestCalcNextChannelManualBalanceResetTime(t *testing.T) {
	loc := time.FixedZone("UTC", 0)
	base := time.Date(2026, 5, 22, 15, 30, 0, 0, loc)

	tests := []struct {
		name   string
		period string
		want   time.Time
	}{
		{
			name:   "daily",
			period: ChannelManualBalanceResetDaily,
			want:   time.Date(2026, 5, 23, 0, 0, 0, 0, loc),
		},
		{
			name:   "weekly",
			period: ChannelManualBalanceResetWeekly,
			want:   time.Date(2026, 5, 25, 0, 0, 0, 0, loc),
		},
		{
			name:   "monthly",
			period: ChannelManualBalanceResetMonthly,
			want:   time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
		},
		{
			name:   "quarterly",
			period: ChannelManualBalanceResetQuarterly,
			want:   time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcNextChannelManualBalanceResetTime(base, tt.period)
			if got != tt.want.Unix() {
				t.Fatalf("got %s, want %s", time.Unix(got, 0).In(loc), tt.want)
			}
		})
	}
}

func TestNormalizeChannelManualBalanceResetPeriod(t *testing.T) {
	if got := NormalizeChannelManualBalanceResetPeriod("unknown"); got != ChannelManualBalanceResetMonthly {
		t.Fatalf("got %q, want %q", got, ChannelManualBalanceResetMonthly)
	}
	if got := NormalizeChannelManualBalanceResetPeriod(ChannelManualBalanceResetQuarterly); got != ChannelManualBalanceResetQuarterly {
		t.Fatalf("got %q, want %q", got, ChannelManualBalanceResetQuarterly)
	}
}

func TestCanParticipateInWeightRandom(t *testing.T) {
	tests := []struct {
		name    string
		channel *Channel
		want    bool
	}{
		{
			name:    "nil channel",
			channel: nil,
			want:    false,
		},
		{
			name: "manual balance disabled always participates",
			channel: &Channel{
				ManualBalanceEnabled: false,
				Balance:              0,
			},
			want: true,
		},
		{
			name: "manual balance enabled below threshold excluded",
			channel: &Channel{
				ManualBalanceEnabled: true,
				Balance:              ChannelWeightRandomMinBalanceUSD - 0.01,
			},
			want: false,
		},
		{
			name: "manual balance enabled at threshold participates",
			channel: &Channel{
				ManualBalanceEnabled: true,
				Balance:              ChannelWeightRandomMinBalanceUSD,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.channel.CanParticipateInWeightRandom()
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
