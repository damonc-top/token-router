package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func InitChannelAffinityLoad() error {
	cache := getChannelAffinityCache()
	if !cache.redisOn() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hash, err := affinityStoreScript.Load(ctx, common.RDB).Result()
	if err != nil {
		return err
	}
	var cursor uint64
	for {
		keys, next, err := common.RDB.Scan(ctx, cursor, channelAffinityCacheNamespace+":*", 256).Result()
		if err != nil {
			return err
		}
		pipe := common.RDB.Pipeline()
		for _, key := range keys {
			pending := affinityPendingPrefix + strings.TrimPrefix(key, channelAffinityCacheNamespace+":")
			pipe.EvalSha(ctx, hash, []string{key, pending}, "get", 1, "", "0", affinityLoadPrefix, 0)
		}
		if len(keys) > 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

type channelAffinityReservation struct {
	lease     affinityLease
	channelID int
	stop      chan struct{}
	done      chan struct{}
	once      sync.Once
}

func holdChannelAffinityReservation(c *gin.Context, lease affinityLease) {
	reservation := &channelAffinityReservation{
		lease:     lease,
		channelID: lease.ChannelID,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	c.Set("channel_affinity_reservation", reservation)
	cache := getChannelAffinityCache()
	var requestDone <-chan struct{}
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	go func() {
		defer close(reservation.done)
		defer func() {
			if err := cache.Release(lease); err != nil {
				common.SysError(common.LogText("channel affinity reservation release failed: %v", err))
			}
		}()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-reservation.stop:
				return
			case <-requestDone:
				return
			case <-ticker.C:
				if err := cache.Renew(lease); err != nil {
					common.SysError(common.LogText("channel affinity reservation renewal failed: %v", err))
				}
			}
		}
	}()
}

func ReleaseChannelAffinityReservation(c *gin.Context) {
	value, ok := c.Get("channel_affinity_reservation")
	if !ok || value == nil {
		return
	}
	reservation := value.(*channelAffinityReservation)
	reservation.once.Do(func() { close(reservation.stop) })
	<-reservation.done
}

func MoveChannelAffinityReservation(c *gin.Context, channelID int) {
	meta, matched := getChannelAffinityMeta(c)
	if !matched || !meta.PreferIdleChannel || RequestPolicy(c).SessionMode == "off" ||
		c.GetBool("channel_affinity_balance_unavailable") {
		return
	}
	if value, ok := c.Get("channel_affinity_reservation"); ok && value != nil {
		reservation := value.(*channelAffinityReservation)
		if reservation.channelID == channelID {
			return
		}
		if err := getChannelAffinityCache().Move(reservation.lease, channelID); err != nil {
			common.SysError(common.LogText("channel affinity reservation move failed: %v", err))
		} else {
			reservation.channelID = channelID
		}
		return
	}
	lease, err := getChannelAffinityCache().Reserve(meta.CacheKey, []affinityCandidate{{ChannelID: channelID, Weight: 1}})
	if err != nil {
		c.Set("channel_affinity_balance_unavailable", true)
		common.SysError(common.LogText("channel affinity reservation acquisition failed: %v", err))
		return
	}
	holdChannelAffinityReservation(c, lease)
}

func resetChannelAffinityReservation(c *gin.Context) {
	ReleaseChannelAffinityReservation(c)
	c.Set("channel_affinity_reservation", nil)
}

func deleteChannelAffinityForRequest(c *gin.Context, key string) (bool, error) {
	cache := getChannelAffinityCache()
	if value, ok := c.Get("channel_affinity_reservation"); ok && value != nil {
		err := cache.Invalidate(value.(*channelAffinityReservation).lease)
		return err == nil, err
	}
	deleted, err := cache.DeleteMany([]string{key})
	return deleted[cache.FullKey(key)], err
}

func selectChannelForSession(param *RetryParam, group string, retry int, filters []dto.ChannelFilter) (*model.Channel, error) {
	meta, matched := getChannelAffinityMeta(param.Ctx)
	if !matched || !meta.PreferIdleChannel || param.GetRetry() != 0 ||
		RequestPolicy(param.Ctx).SessionMode == "off" || param.Ctx.GetBool("channel_affinity_balance_unavailable") {
		return model.GetRandomSatisfiedChannel(group, param.ModelName, retry, filters)
	}
	if value, ok := param.Ctx.Get("channel_affinity_reservation"); ok && value != nil {
		reservation := value.(*channelAffinityReservation)
		return model.CacheGetChannel(reservation.channelID)
	}
	candidates, err := model.GetWeightedChannelCandidates(group, param.ModelName, retry, filters)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	lease, err := getChannelAffinityCache().Reserve(meta.CacheKey, candidates)
	if err != nil {
		param.Ctx.Set("channel_affinity_balance_unavailable", true)
		common.SysError(common.LogText("channel affinity idle selection failed: %v", err))
		return model.GetRandomSatisfiedChannel(group, param.ModelName, retry, filters)
	}
	channel, err := model.CacheGetChannel(lease.ChannelID)
	if err != nil || channel == nil || channel.Status != common.ChannelStatusEnabled {
		_ = getChannelAffinityCache().Release(lease)
		return nil, errors.New("reserved session channel is unavailable")
	}
	if usable, _ := model.ChannelSatisfiesFilters(channel, param.ModelName, filters); !usable ||
		!model.IsChannelEnabledForGroupModel(group, param.ModelName, channel.Id) {
		_ = getChannelAffinityCache().Release(lease)
		return nil, errors.New("reserved session channel cannot serve this request")
	}
	holdChannelAffinityReservation(param.Ctx, lease)
	return channel, nil
}
