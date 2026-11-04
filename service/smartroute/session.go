package smartroute

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
)

const (
	sessionCacheNamespace  = "new-api:smart_routing_session:v1"
	sessionCacheCapacity   = 100_000
	sessionCacheTTLSeconds = 3600
)

type SessionState struct {
	CurrentRoute   string `json:"current_route"`
	LastNonAdvisor string `json:"last_non_advisor"`
	LastChannelID  int    `json:"last_channel_id"`
}

var (
	sessionCache     *cachex.HybridCache[SessionState]
	sessionCacheOnce sync.Once
)

func getSessionCache() *cachex.HybridCache[SessionState] {
	sessionCacheOnce.Do(func() {
		sessionCache = cachex.NewHybridCache[SessionState](cachex.HybridCacheConfig[SessionState]{
			Namespace: cachex.Namespace(sessionCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SessionState]{},
			Memory: func() *hot.HotCache[string, SessionState] {
				return hot.NewHotCache[string, SessionState](hot.LRU, sessionCacheCapacity).
					WithTTL(sessionCacheTTLSeconds * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return sessionCache
}

func LoadSession(sessionKey string) (SessionState, bool) {
	if sessionKey == "" {
		return SessionState{}, false
	}
	state, found, err := getSessionCache().Get(sessionKey)
	if err != nil || !found {
		return SessionState{}, false
	}
	return state, true
}

func SaveSessionAfterSuccess(sessionKey string, isUserTurn bool, route string, channelID int) {
	if sessionKey == "" {
		return
	}
	state, _, err := getSessionCache().Get(sessionKey)
	if err != nil {
		common.SysError(common.LogText("smart routing session load failed, skipping save: %s", err.Error()))
		return
	}
	state.LastChannelID = channelID
	if isUserTurn {
		state.CurrentRoute = route
		if route != RouteAdvisor {
			state.LastNonAdvisor = route
		}
	}
	if err := getSessionCache().SetWithTTL(sessionKey, state, sessionCacheTTLSeconds*time.Second); err != nil {
		common.SysError(common.LogText("smart routing session save failed: %s", err.Error()))
	}
}
