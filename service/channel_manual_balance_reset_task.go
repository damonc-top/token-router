package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	channelManualBalanceResetTickInterval = 5 * time.Minute
	channelManualBalanceResetBatchSize    = 300
)

var (
	channelManualBalanceResetOnce    sync.Once
	channelManualBalanceResetRunning atomic.Bool
)

func StartChannelManualBalanceResetTask() {
	channelManualBalanceResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("channel manual balance reset task started: tick=%s", channelManualBalanceResetTickInterval))
			ticker := time.NewTicker(channelManualBalanceResetTickInterval)
			defer ticker.Stop()

			runChannelManualBalanceResetOnce()
			for range ticker.C {
				runChannelManualBalanceResetOnce()
			}
		})
	})
}

func runChannelManualBalanceResetOnce() {
	if !channelManualBalanceResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelManualBalanceResetRunning.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	totalReset := 0
	for {
		if ctx.Err() != nil {
			logger.LogWarn(ctx, "channel manual balance reset task timed out")
			return
		}
		n, err := model.ResetDueChannelManualBalances(channelManualBalanceResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("channel manual balance reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < channelManualBalanceResetBatchSize {
			break
		}
	}
	if common.DebugEnabled && totalReset > 0 {
		logger.LogDebug(ctx, "channel manual balance reset: reset_count=%d", totalReset)
	}
}
