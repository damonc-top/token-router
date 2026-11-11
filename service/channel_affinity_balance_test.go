package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAffinityBalanceTestStore(t *testing.T, backend string, capacity int) (*channelAffinityStore, func(time.Duration)) {
	t.Helper()
	previousRedis, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = nil, false
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	var server *miniredis.Miniredis
	if backend == "redis" {
		server = miniredis.RunT(t)
		server.SetTime(now)
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		common.RDB, common.RedisEnabled = client, true
		t.Cleanup(func() { assert.NoError(t, client.Close()) })
	}
	t.Cleanup(func() { common.RDB, common.RedisEnabled = previousRedis, previousEnabled })
	store := newChannelAffinityStore(capacity)
	store.now = func() time.Time { return now }
	return store, func(elapsed time.Duration) {
		now = now.Add(elapsed)
		if server != nil {
			server.FastForward(elapsed)
			server.SetTime(now)
		}
	}
}

func TestAffinityBalanceConcurrentFirstSessions(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			store, _ := newAffinityBalanceTestStore(t, backend, 100)
			candidates := []affinityCandidate{{1, 100}, {2, 100}, {3, 100}, {4, 100}}
			var leases [4]affinityLease
			var failures [4]error
			start := make(chan struct{})
			var workers sync.WaitGroup
			for i := range leases {
				workers.Go(func() {
					<-start
					leases[i], failures[i] = store.Reserve(fmt.Sprintf("session-%d", i), candidates)
				})
			}
			close(start)
			workers.Wait()
			assigned := make(map[int]bool)
			for i, lease := range leases {
				require.NoError(t, failures[i])
				assert.False(t, assigned[lease.ChannelID], "each first session must occupy a different idle channel")
				assigned[lease.ChannelID] = true
			}
			assert.Len(t, assigned, 4)
			fifth, err := store.Reserve("fifth", candidates)
			require.NoError(t, err)
			assert.Contains(t, assigned, fifth.ChannelID, "all occupied channels remain eligible")
			for _, lease := range leases {
				require.NoError(t, store.Release(lease))
			}
			require.NoError(t, store.Release(fifth))
			keys, err := store.Keys()
			require.NoError(t, err)
			assert.Empty(t, keys, "failed initial requests must release their reservations")
		})
	}
}

func TestAffinityBalanceSharedReservationAndIdleWeights(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			store, _ := newAffinityBalanceTestStore(t, backend, 100)
			first, err := store.Reserve("shared", []affinityCandidate{{1, 100}})
			require.NoError(t, err)
			second, err := store.Reserve("shared", []affinityCandidate{{1, 100}, {2, 100}})
			require.NoError(t, err)
			assert.Equal(t, first, second)
			require.NoError(t, store.Release(first))
			next, err := store.Reserve("next", []affinityCandidate{{1, 1000000}, {2, 1}, {3, 0}})
			require.NoError(t, err)
			assert.Equal(t, 2, next.ChannelID, "idle wins over the occupied channel's higher weight")
			require.NoError(t, store.Release(next))
			require.NoError(t, store.Commit(second, 1, 30*time.Minute))
			require.NoError(t, store.Release(second))
			fallback, err := store.Reserve("fallback", []affinityCandidate{{1, 100}, {3, 0}})
			require.NoError(t, err)
			assert.Equal(t, 1, fallback.ChannelID, "a zero-weight idle channel cannot gain eligibility")
			require.NoError(t, store.Release(fallback))
			id, found, err := store.Get("shared")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, 1, id)
		})
	}
}

func TestAffinityBalanceSlidingExpiryAndLongRequest(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			store, advance := newAffinityBalanceTestStore(t, backend, 100)
			require.NoError(t, store.SetWithTTL("active", 1, time.Minute))
			active, err := store.Reserve("active", []affinityCandidate{{1, 1}})
			require.NoError(t, err)
			advance(90 * time.Second)
			require.NoError(t, store.Renew(active))
			_, found, err := store.Get("active")
			require.NoError(t, err)
			assert.False(t, found, "renewal preserves in-flight occupation without extending the confirmed binding")
			other, err := store.Reserve("other", []affinityCandidate{{1, 1}, {2, 1}})
			require.NoError(t, err)
			assert.Equal(t, 2, other.ChannelID)
			require.NoError(t, store.Commit(active, 1, 30*time.Minute))
			require.NoError(t, store.Release(active))
			require.NoError(t, store.Release(other))
			advance(29 * time.Minute)
			active, err = store.Reserve("active", []affinityCandidate{{1, 1}})
			require.NoError(t, err)
			require.NoError(t, store.Commit(active, 1, 30*time.Minute))
			require.NoError(t, store.Release(active))
			advance(2 * time.Minute)
			_, found, err = store.Get("active")
			require.NoError(t, err)
			assert.True(t, found, "a successful follow-up request restarts the 30-minute lifetime")
			advance(28 * time.Minute)
			_, found, err = store.Get("active")
			require.NoError(t, err)
			assert.False(t, found)
		})
	}
}

func TestAffinityBalanceMoveAndFencing(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			store, advance := newAffinityBalanceTestStore(t, backend, 100)
			require.NoError(t, store.SetWithTTL("moving", 1, 30*time.Minute))
			moving, err := store.Reserve("moving", []affinityCandidate{{1, 1}})
			require.NoError(t, err)
			require.NoError(t, store.Move(moving, 2))
			id, found, err := store.Get("moving")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, 1, id, "retry occupation must preserve the previous confirmed binding until success")
			other, err := store.Reserve("other", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}})
			require.NoError(t, err)
			assert.Equal(t, 3, other.ChannelID, "both the binding and retry must count as occupied")
			require.NoError(t, store.Commit(moving, 2, 30*time.Minute))
			require.NoError(t, store.Release(moving))
			require.NoError(t, store.Invalidate(moving))
			free, err := store.Reserve("free", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}})
			require.NoError(t, err)
			assert.Equal(t, 1, free.ChannelID)
			_, err = store.DeleteMany([]string{"moving"})
			require.NoError(t, err)
			replacement, err := store.Reserve("moving", []affinityCandidate{{4, 1}})
			require.NoError(t, err)
			require.NoError(t, store.Commit(replacement, 4, 30*time.Minute))
			require.NoError(t, store.Commit(moving, 2, 30*time.Minute))
			require.NoError(t, store.Release(moving))
			id, found, err = store.Get("moving")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, 4, id, "late completion must not overwrite a replacement generation")
			require.NoError(t, store.Release(replacement))
			require.NoError(t, store.Release(free))
			require.NoError(t, store.Release(other))
			expired, err := store.Reserve("expired", []affinityCandidate{{5, 1}})
			require.NoError(t, err)
			advance(affinityLeaseTTL + time.Second)
			require.NoError(t, store.Commit(expired, 5, 30*time.Minute))
			_, found, err = store.Get("expired")
			require.NoError(t, err)
			assert.False(t, found, "expired reservations cannot be resurrected")
		})
	}
}

func TestAffinityBalanceSharedMigrationKeepsInFlightChannelsOccupied(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			store, _ := newAffinityBalanceTestStore(t, backend, 100)
			first, err := store.Reserve("shared", []affinityCandidate{{1, 1}})
			require.NoError(t, err)
			second, err := store.Reserve("shared", []affinityCandidate{{1, 1}, {2, 1}})
			require.NoError(t, err)
			require.NoError(t, store.Move(first, 2))
			other, err := store.Reserve("other", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}})
			require.NoError(t, err)
			assert.Equal(t, 3, other.ChannelID, "the parallel request still occupies the original channel")
			require.NoError(t, store.Commit(first, 2, 30*time.Minute))
			require.NoError(t, store.Release(first))
			next, err := store.Reserve("next", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}, {4, 1}})
			require.NoError(t, err)
			assert.Equal(t, 4, next.ChannelID, "successful migration must retain the parallel request's occupation")
			require.NoError(t, store.Release(second))
			free, err := store.Reserve("free", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}, {4, 1}})
			require.NoError(t, err)
			assert.Equal(t, 1, free.ChannelID, "the final release frees old channels while preserving the new binding")
		})
	}
}

func TestAffinityBalanceClearAndCapacity(t *testing.T) {
	store, _ := newAffinityBalanceTestStore(t, "memory", 2)
	require.NoError(t, store.SetWithTTL("rule:a", 1, time.Hour))
	require.NoError(t, store.SetWithTTL("rule:b", 2, time.Hour))
	_, _, err := store.Get("rule:a")
	require.NoError(t, err)
	require.NoError(t, store.SetWithTTL("rule:c", 3, time.Hour))
	free, err := store.Reserve("rule:d", []affinityCandidate{{1, 1}, {2, 1}, {3, 1}})
	require.NoError(t, err)
	assert.Equal(t, 2, free.ChannelID, "capacity eviction must release the old occupation")
	deleted, err := store.DeleteByPrefix("rule")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
	require.NoError(t, store.Commit(free, 2, time.Hour))
	keys, err := store.Keys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestAffinityBalanceRequestCancellationReleasesPreselection(t *testing.T) {
	getChannelAffinityCache()
	previousCache := channelAffinityCache
	store, _ := newAffinityBalanceTestStore(t, "memory", 10)
	channelAffinityCache = store
	t.Cleanup(func() { channelAffinityCache = previousCache })
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext, cancel := context.WithCancel(context.Background())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)
	lease, err := store.Reserve("preselected", []affinityCandidate{{1, 1}})
	require.NoError(t, err)
	holdChannelAffinityReservation(ctx, lease)
	value, _ := ctx.Get("channel_affinity_reservation")
	reservation := value.(*channelAffinityReservation)
	t.Cleanup(func() { cancel(); ReleaseChannelAffinityReservation(ctx) })
	cancel()
	select {
	case <-reservation.done:
	case <-t.Context().Done():
		t.Fatal("request cancellation did not finish its reservation")
	}
	keys, err := store.Keys()
	require.NoError(t, err)
	assert.Empty(t, keys, "preselection must release even if later middleware never reaches Distribute")
}

func TestAffinityBalanceRestoresLegacyRedisOccupancy(t *testing.T) {
	getChannelAffinityCache()
	previousCache := channelAffinityCache
	store, _ := newAffinityBalanceTestStore(t, "redis", 10)
	channelAffinityCache = store
	t.Cleanup(func() { channelAffinityCache = previousCache })
	require.NoError(t, common.RDB.Set(t.Context(), store.FullKey("legacy:active"), 1, 30*time.Minute).Err())
	require.NoError(t, InitChannelAffinityLoad())
	lease, err := store.Reserve("legacy:new", []affinityCandidate{{1, 1}, {2, 1}})
	require.NoError(t, err)
	assert.Equal(t, 2, lease.ChannelID, "upgrading must account for previously stored session bindings")
	deleted, err := store.DeleteByPrefix("legacy")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted, "clearing a rule includes its pending reservations")
	require.NoError(t, store.Commit(lease, 2, 30*time.Minute))
	keys, err := store.Keys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestAffinityBalanceRoutingKeepsSessionsAndReleasesFailures(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			db := setupChannelSelectAutoGroupsTest(t)
			for id := 1; id <= 4; id++ {
				createChannelSelectAutoGroupsChannel(t, db, id, "default", "gpt-balanced")
			}
			model.InitChannelCache()
			getChannelAffinityCache()
			previousCache := channelAffinityCache
			store, _ := newAffinityBalanceTestStore(t, backend, 100)
			channelAffinityCache = store
			t.Cleanup(func() { channelAffinityCache = previousCache })
			setting := operation_setting.GetChannelAffinitySetting()
			previousSetting := *setting
			*setting = operation_setting.ChannelAffinitySetting{
				Enabled: true, SwitchOnSuccess: true, DefaultTTLSeconds: 1800,
				Rules: []operation_setting.ChannelAffinityRule{{
					Name: "balanced", ModelRegex: []string{"^gpt-balanced$"},
					KeySources:        []operation_setting.ChannelAffinityKeySource{{Type: "request_header", Key: "X-Session"}},
					PreferIdleChannel: true, IncludeRuleName: true, SessionMode: "prefer",
				}},
			}
			t.Cleanup(func() { *setting = previousSetting })
			selectSession := func(session string) (*gin.Context, int) {
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				ctx.Request.Header.Set("X-Session", session)
				channel, _, selectErr := SelectChannelForRequest(ctx, "gpt-balanced", &RetryParam{
					Ctx: ctx, TokenGroup: "default", ModelName: "gpt-balanced", Retry: common.GetPointer(0),
				})
				require.Nil(t, selectErr)
				require.NotNil(t, channel)
				ctx.Set("channel_id", channel.Id)
				MoveChannelAffinityReservation(ctx, channel.Id)
				t.Cleanup(func() { ReleaseChannelAffinityReservation(ctx) })
				return ctx, channel.Id
			}
			first, firstID := selectSession("first")
			second, secondID := selectSession("second")
			assert.NotEqual(t, firstID, secondID)
			RecordChannelAffinity(first, firstID)
			ReleaseChannelAffinityReservation(first)
			followup, followupID := selectSession("first")
			assert.Equal(t, firstID, followupID)
			ReleaseChannelAffinityReservation(followup)
			RequestPolicy(second).FinalLogged = true
			RecordChannelAffinity(second, secondID)
			ReleaseChannelAffinityReservation(second)
			_, found, err := store.Get("balanced:second")
			require.NoError(t, err)
			assert.False(t, found, "an SSE failure with committed HTTP 200 must not establish affinity")
			cancelled, cancelledID := selectSession("cancelled")
			cancelContext, cancel := context.WithCancel(cancelled.Request.Context())
			cancelled.Request = cancelled.Request.WithContext(cancelContext)
			cancel()
			RecordChannelAffinity(cancelled, cancelledID)
			ReleaseChannelAffinityReservation(cancelled)
			_, found, err = store.Get("balanced:cancelled")
			require.NoError(t, err)
			assert.False(t, found)
			setting.KeepOnChannelDisabled = true
			require.NoError(t, store.SetWithTTL("balanced:unavailable", 99, 30*time.Minute))
			recovered, recoveredID := selectSession("unavailable")
			assert.NotEqual(t, 99, recoveredID)
			RecordChannelAffinity(recovered, recoveredID)
			ReleaseChannelAffinityReservation(recovered)
			id, found, err := store.Get("balanced:unavailable")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, recoveredID, id, "prefer mode may replace an unusable preserved binding after success")
			old, _ := selectSession("late")
			parallel, _ := selectSession("late")
			InvalidateChannelAffinity(old)
			replacement, replacementID := selectSession("late")
			RecordChannelAffinity(replacement, replacementID)
			TryInvalidateChannelAffinityOnError(parallel, types.NewErrorWithStatusCode(
				fmt.Errorf("upstream rejected"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway,
			))
			id, found, err = store.Get("balanced:late")
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, replacementID, id, "a late failure must not clear a newer successful session")
		})
	}
}
