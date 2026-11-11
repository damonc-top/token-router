package service

import (
	"container/heap"
	"container/list"
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const affinityLeaseTTL = 2 * time.Minute
const affinityPendingPrefix = "new-api:channel_affinity_pending:v1:"
const affinityLoadPrefix = "new-api:channel_affinity_load:v1:"

type affinityCandidate = model.WeightedChannel

type affinityLease struct {
	Key       string
	ChannelID int
	Owner     string
}

type affinityEntry struct {
	key            string
	channelID      int
	leaseChannelID int
	leaseChannels  []int
	boundUntil     time.Time
	leaseUntil     time.Time
	owner          string
	references     int
	expiryIndex    int
	lruElement     *list.Element
}

func (e *affinityEntry) expiresAt() time.Time {
	if e.boundUntil.IsZero() || !e.leaseUntil.IsZero() && e.leaseUntil.Before(e.boundUntil) {
		return e.leaseUntil
	}
	return e.boundUntil
}

type affinityExpiryHeap []*affinityEntry

func (h affinityExpiryHeap) Len() int           { return len(h) }
func (h affinityExpiryHeap) Less(i, j int) bool { return h[i].expiresAt().Before(h[j].expiresAt()) }
func (h affinityExpiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].expiryIndex, h[j].expiryIndex = i, j
}
func (h *affinityExpiryHeap) Push(value any) {
	e := value.(*affinityEntry)
	e.expiryIndex = len(*h)
	*h = append(*h, e)
}
func (h *affinityExpiryHeap) Pop() any {
	old := *h
	e := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	e.expiryIndex = -1
	return e
}

type channelAffinityStore struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*affinityEntry
	load     map[int]int
	expiry   affinityExpiryHeap
	lru      list.List
	now      func() time.Time
}

func newChannelAffinityStore(capacity int) *channelAffinityStore {
	if capacity <= 0 {
		capacity = 100_000
	}
	return &channelAffinityStore{
		capacity: capacity,
		entries:  make(map[string]*affinityEntry),
		load:     make(map[int]int),
		now:      time.Now,
	}
}

func (s *channelAffinityStore) FullKey(key string) string {
	return cachex.Namespace(channelAffinityCacheNamespace).FullKey(key)
}

func (s *channelAffinityStore) redisOn() bool {
	return common.RedisEnabled && common.RDB != nil
}

func (s *channelAffinityStore) remove(e *affinityEntry) {
	delete(s.entries, e.key)
	s.track(e, -1)
	heap.Remove(&s.expiry, e.expiryIndex)
	s.lru.Remove(e.lruElement)
}

func (s *channelAffinityStore) track(e *affinityEntry, delta int) {
	bound := 0
	if !e.boundUntil.IsZero() {
		bound = e.channelID
		s.load[bound] += delta
		if s.load[bound] == 0 {
			delete(s.load, bound)
		}
	}
	if e.leaseUntil.IsZero() {
		return
	}
	for _, id := range e.leaseChannels {
		if id > 0 && id != bound {
			s.load[id] += delta
			if s.load[id] == 0 {
				delete(s.load, id)
			}
		}
	}
}

func (s *channelAffinityStore) prune(now time.Time) {
	for len(s.expiry) > 0 {
		e := s.expiry[0]
		if e.expiresAt().After(now) {
			break
		}
		s.track(e, -1)
		if !e.boundUntil.After(now) {
			e.boundUntil = time.Time{}
		}
		if !e.leaseUntil.After(now) {
			e.leaseUntil, e.owner, e.references = time.Time{}, "", 0
			e.leaseChannelID = 0
			e.leaseChannels = nil
		}
		s.track(e, 1)
		if e.boundUntil.IsZero() && e.leaseUntil.IsZero() {
			s.remove(e)
		} else {
			heap.Fix(&s.expiry, e.expiryIndex)
		}
	}
}

func (s *channelAffinityStore) add(key string, channelID int) (*affinityEntry, error) {
	if len(s.entries) >= s.capacity {
		oldest := s.lru.Back()
		for oldest != nil && oldest.Value.(*affinityEntry).references > 0 {
			oldest = oldest.Prev()
		}
		if oldest == nil {
			return nil, errors.New("channel affinity reservation capacity reached")
		}
		s.remove(oldest.Value.(*affinityEntry))
	}
	e := &affinityEntry{key: key, channelID: channelID, expiryIndex: -1}
	e.lruElement = s.lru.PushFront(e)
	s.entries[key] = e
	return e, nil
}

var affinityStoreScript = redis.NewScript(`
local clock = redis.call('TIME')
local op, now, ttl = ARGV[1], tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000), tonumber(ARGV[2])
local owner, channel = ARGV[3], ARGV[4]
local key, pending = KEYS[1], KEYS[2]
local prefix = ARGV[5]
local bound = redis.call('GET', key)
local p = redis.call('HMGET', pending, 'channel', 'owner', 'refs', 'until', 'channels')
local held = {}
if p[1] then
  p[5] = p[5] or p[1]
  for id in string.gmatch(p[5], '[^,]+') do held[id] = true end
end
local function remove_load(id)
  if id and id ~= '' then redis.call('ZREM', prefix .. id, key) end
end
local function refresh(id)
  if not id or id == '' then return end
  local until_time = 0
  if bound == id then
    local remaining = redis.call('PTTL', key)
    if remaining > 0 then until_time = now + remaining end
  end
  if held[id] and p[4] then until_time = math.max(until_time, tonumber(p[4])) end
  if until_time <= now then remove_load(id); return end
  local load_key = prefix .. id
  redis.call('ZADD', load_key, until_time, key)
  local remaining = redis.call('PTTL', load_key)
  if remaining < until_time - now then redis.call('PEXPIRE', load_key, until_time - now) end
end
local function refresh_pending()
  for id in pairs(held) do refresh(id) end
  if bound and not held[bound] then refresh(bound) end
end
local function clear_pending()
  local previous = held
  redis.call('DEL', pending)
  p, held = {}, {}
  for id in pairs(previous) do refresh(id) end
end
local function touch(id)
  if not held[id] then
    held[id] = true
    p[5] = p[5] and p[5] .. ',' .. id or id
    redis.call('HSET', pending, 'channels', p[5])
  end
end
if p[4] and tonumber(p[4]) <= now then
  clear_pending()
end
if op == 'get' then
  if bound then refresh(bound); return tonumber(bound) end
  return 0
end
if op == 'invalidate' and (not p[2] or p[2] ~= owner) then return 0 end
if op == 'delete' or op == 'invalidate' then
  local existed = bound or p[1]
  remove_load(bound)
  redis.call('DEL', key)
  bound = false
  clear_pending()
  if existed then return 1 end
  return 0
end
if op == 'set' then
  remove_load(bound)
  bound = channel
  redis.call('SET', key, channel, 'PX', ttl)
  clear_pending()
  refresh(channel)
  return 1
end
if op == 'reserve' then
  local allowed = {}
  for i = 7, #ARGV, 2 do allowed[ARGV[i]] = true end
  if p[1] and not allowed[p[1]] then
    clear_pending()
  end
  if p[1] then
    redis.call('HINCRBY', pending, 'refs', 1)
    p[4] = now + ttl
    redis.call('HSET', pending, 'until', p[4])
    redis.call('PEXPIRE', pending, ttl)
    refresh_pending()
    return {tonumber(p[1]), p[2]}
  end
  if bound and allowed[bound] then
    channel = bound
  else
    local candidates, weight, idle = {}, 0, false
    for i = 7, #ARGV, 2 do
      local id, w = ARGV[i], tonumber(ARGV[i+1])
      local load_key = prefix .. id
      redis.call('ZREMRANGEBYSCORE', load_key, '-inf', now)
      local free = redis.call('ZCARD', load_key) == 0
      if free and not idle then candidates = {}; weight = 0; idle = true end
      if free or not idle then
        candidates[#candidates+1] = {id, w}
        weight = weight + w
      end
    end
    if weight <= 0 then return {0, ''} end
    local ticket = tonumber(ARGV[6]) * weight
    for _, candidate in ipairs(candidates) do
      ticket = ticket - candidate[2]
      if ticket < 0 then channel = candidate[1]; break end
    end
  end
  p = {channel, owner, 1, now + ttl, channel}
  held = {[channel] = true}
  redis.call('HSET', pending, 'channel', channel, 'owner', owner, 'refs', 1, 'until', p[4], 'channels', channel)
  redis.call('PEXPIRE', pending, ttl)
  refresh_pending()
  return {tonumber(channel), owner}
end
if not p[2] or p[2] ~= owner then return 0 end
if op == 'release' then
  if tonumber(p[3]) > 1 then
    redis.call('HINCRBY', pending, 'refs', -1)
  else
    clear_pending()
  end
  return 1
end
if op == 'commit' then
  local previous = bound
  bound = channel
  redis.call('SET', key, channel, 'PX', ttl)
  p[1] = channel
  redis.call('HSET', pending, 'channel', channel)
  touch(channel)
  refresh(previous)
elseif op == 'move' then
  p[1] = channel
  redis.call('HSET', pending, 'channel', channel)
  touch(channel)
elseif op == 'renew' then
  p[4] = now + ttl
  redis.call('HSET', pending, 'until', p[4])
  redis.call('PEXPIRE', pending, ttl)
end
refresh_pending()
return 1
`)

func (s *channelAffinityStore) run(op, key, owner string, channelID int, ttl time.Duration, candidates []affinityCandidate) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := []any{op, max(ttl.Milliseconds(), 1), owner, strconv.Itoa(channelID), affinityLoadPrefix, rand.Float64()}
	for _, candidate := range candidates {
		if candidate.Weight > 0 {
			args = append(args, strconv.Itoa(candidate.ChannelID), candidate.Weight)
		}
	}
	return affinityStoreScript.Run(ctx, common.RDB, []string{key, affinityPendingPrefix + strings.TrimPrefix(key, channelAffinityCacheNamespace+":")}, args...).Result()
}

func (s *channelAffinityStore) Get(key string) (int, bool, error) {
	key = s.FullKey(key)
	if key == "" {
		return 0, false, nil
	}
	if s.redisOn() {
		result, err := s.run("get", key, "", 0, 0, nil)
		if err != nil {
			return 0, false, err
		}
		channelID := int(result.(int64))
		return channelID, channelID > 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.now())
	if e := s.entries[key]; e != nil && !e.boundUntil.IsZero() {
		s.lru.MoveToFront(e.lruElement)
		return e.channelID, true, nil
	}
	return 0, false, nil
}

func (s *channelAffinityStore) SetWithTTL(key string, channelID int, ttl time.Duration) error {
	key = s.FullKey(key)
	if key == "" || channelID <= 0 {
		return nil
	}
	if s.redisOn() {
		_, err := s.run("set", key, "", channelID, ttl, nil)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.prune(now)
	e := s.entries[key]
	if e == nil {
		var err error
		e, err = s.add(key, channelID)
		if err != nil {
			return err
		}
		e.boundUntil = now.Add(ttl)
		s.track(e, 1)
		heap.Push(&s.expiry, e)
	} else {
		s.track(e, -1)
		e.channelID, e.leaseChannelID = channelID, 0
		e.leaseChannels = nil
		e.boundUntil, e.leaseUntil = now.Add(ttl), time.Time{}
		e.owner, e.references = "", 0
		s.track(e, 1)
		s.lru.MoveToFront(e.lruElement)
		heap.Fix(&s.expiry, e.expiryIndex)
	}
	return nil
}

func (s *channelAffinityStore) Reserve(key string, candidates []affinityCandidate) (affinityLease, error) {
	key = s.FullKey(key)
	if key == "" || len(candidates) == 0 {
		return affinityLease{}, nil
	}
	owner := uuid.NewString()
	if s.redisOn() {
		result, err := s.run("reserve", key, owner, 0, affinityLeaseTTL, candidates)
		if err != nil {
			return affinityLease{}, err
		}
		values := result.([]any)
		return affinityLease{Key: key, ChannelID: int(values[0].(int64)), Owner: values[1].(string)}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.prune(now)
	e := s.entries[key]
	channelID := 0
	if e != nil {
		if e.owner != "" && slices.ContainsFunc(candidates, func(candidate affinityCandidate) bool {
			return candidate.ChannelID == e.leaseChannelID && candidate.Weight > 0
		}) {
			channelID, owner = e.leaseChannelID, e.owner
		} else if !e.boundUntil.IsZero() && slices.ContainsFunc(candidates, func(candidate affinityCandidate) bool {
			return candidate.ChannelID == e.channelID && candidate.Weight > 0
		}) {
			channelID = e.channelID
		}
	}
	if channelID == 0 {
		idle := false
		total := 0
		for _, candidate := range candidates {
			if candidate.Weight > 0 && s.load[candidate.ChannelID] == 0 {
				idle = true
				break
			}
		}
		for _, candidate := range candidates {
			if candidate.Weight > 0 && (!idle || s.load[candidate.ChannelID] == 0) {
				total += candidate.Weight
			}
		}
		if total <= 0 {
			return affinityLease{}, nil
		}
		ticket := rand.IntN(total)
		for _, candidate := range candidates {
			if candidate.Weight <= 0 || idle && s.load[candidate.ChannelID] != 0 {
				continue
			}
			ticket -= candidate.Weight
			if ticket < 0 {
				channelID = candidate.ChannelID
				break
			}
		}
	}
	created := e == nil
	if created {
		var err error
		e, err = s.add(key, channelID)
		if err != nil {
			return affinityLease{}, err
		}
	} else {
		s.track(e, -1)
	}
	if e.owner != owner {
		e.references = 0
		e.leaseChannels = nil
	}
	e.owner, e.leaseChannelID = owner, channelID
	if !slices.Contains(e.leaseChannels, channelID) {
		e.leaseChannels = append(e.leaseChannels, channelID)
	}
	e.references++
	e.leaseUntil = now.Add(affinityLeaseTTL)
	s.track(e, 1)
	s.lru.MoveToFront(e.lruElement)
	if created {
		heap.Push(&s.expiry, e)
	} else {
		heap.Fix(&s.expiry, e.expiryIndex)
	}
	return affinityLease{Key: key, ChannelID: e.leaseChannelID, Owner: e.owner}, nil
}

func (s *channelAffinityStore) updateLease(op string, lease affinityLease, channelID int, ttl time.Duration) error {
	if lease.Owner == "" {
		return nil
	}
	if s.redisOn() {
		_, err := s.run(op, s.FullKey(lease.Key), lease.Owner, channelID, ttl, nil)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.prune(now)
	e := s.entries[s.FullKey(lease.Key)]
	if e == nil || e.owner != lease.Owner {
		return nil
	}
	if op == "invalidate" {
		s.remove(e)
		return nil
	}
	s.track(e, -1)
	switch op {
	case "release":
		e.references--
		if e.references <= 0 {
			e.owner, e.leaseUntil = "", time.Time{}
			e.leaseChannelID = 0
			e.leaseChannels = nil
			if e.boundUntil.IsZero() {
				s.remove(e)
				return nil
			}
		}
	case "renew":
		e.leaseUntil = now.Add(affinityLeaseTTL)
	case "commit":
		e.channelID, e.leaseChannelID = channelID, channelID
		e.boundUntil = now.Add(ttl)
		if !slices.Contains(e.leaseChannels, channelID) {
			e.leaseChannels = append(e.leaseChannels, channelID)
		}
	case "move":
		e.leaseChannelID = channelID
		if !slices.Contains(e.leaseChannels, channelID) {
			e.leaseChannels = append(e.leaseChannels, channelID)
		}
	}
	s.track(e, 1)
	s.lru.MoveToFront(e.lruElement)
	heap.Fix(&s.expiry, e.expiryIndex)
	return nil
}

func (s *channelAffinityStore) Renew(lease affinityLease) error {
	return s.updateLease("renew", lease, 0, affinityLeaseTTL)
}

func (s *channelAffinityStore) Release(lease affinityLease) error {
	return s.updateLease("release", lease, 0, 0)
}

func (s *channelAffinityStore) Commit(lease affinityLease, channelID int, ttl time.Duration) error {
	return s.updateLease("commit", lease, channelID, ttl)
}

func (s *channelAffinityStore) Move(lease affinityLease, channelID int) error {
	return s.updateLease("move", lease, channelID, affinityLeaseTTL)
}

func (s *channelAffinityStore) Invalidate(lease affinityLease) error {
	return s.updateLease("invalidate", lease, 0, 0)
}

func (s *channelAffinityStore) Keys() ([]string, error) {
	if s.redisOn() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		keys := make(map[string]struct{})
		for _, prefix := range []string{channelAffinityCacheNamespace + ":", affinityPendingPrefix} {
			var cursor uint64
			for {
				batch, next, err := common.RDB.Scan(ctx, cursor, prefix+"*", 1000).Result()
				if err != nil {
					return nil, err
				}
				for _, key := range batch {
					keys[s.FullKey(strings.TrimPrefix(key, prefix))] = struct{}{}
				}
				cursor = next
				if cursor == 0 {
					break
				}
			}
		}
		result := make([]string, 0, len(keys))
		for key := range keys {
			result = append(result, key)
		}
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.now())
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *channelAffinityStore) DeleteMany(keys []string) (map[string]bool, error) {
	result := make(map[string]bool, len(keys))
	if s.redisOn() {
		if len(keys) == 0 {
			return result, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client := common.RDB
		if err := affinityStoreScript.Load(ctx, client).Err(); err != nil {
			return result, err
		}
		for start := 0; start < len(keys); start += 256 {
			batch := keys[start:min(start+256, len(keys))]
			pipe := client.Pipeline()
			commands := make(map[string]*redis.Cmd, len(batch))
			for _, key := range batch {
				key = s.FullKey(key)
				if key != "" {
					pending := affinityPendingPrefix + strings.TrimPrefix(key, channelAffinityCacheNamespace+":")
					commands[key] = affinityStoreScript.EvalSha(ctx, pipe, []string{key, pending}, "delete", 1, "", "0", affinityLoadPrefix, 0)
				}
			}
			if _, err := pipe.Exec(ctx); err != nil {
				_ = pipe.Close()
				return result, err
			}
			_ = pipe.Close()
			for key, command := range commands {
				deleted, err := command.Int64()
				if err != nil {
					return result, err
				}
				result[key] = deleted > 0 || result[key]
			}
		}
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(s.now())
	for _, key := range keys {
		key = s.FullKey(key)
		e := s.entries[key]
		result[key] = e != nil
		if e != nil {
			s.remove(e)
		}
	}
	return result, nil
}

func (s *channelAffinityStore) DeleteByPrefix(prefix string) (int, error) {
	prefix = s.FullKey(prefix) + ":"
	keys, err := s.Keys()
	if err != nil {
		return 0, err
	}
	filtered := keys[:0]
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			filtered = append(filtered, key)
		}
	}
	deleted, err := s.DeleteMany(filtered)
	count := 0
	for _, ok := range deleted {
		if ok {
			count++
		}
	}
	return count, err
}

func (s *channelAffinityStore) Capacity() (int, int) {
	if s.redisOn() {
		return 0, 0
	}
	return s.capacity, 0
}

func (s *channelAffinityStore) Algorithm() (string, string) {
	if s.redisOn() {
		return "redis", ""
	}
	return "lru", ""
}
