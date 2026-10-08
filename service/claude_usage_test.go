package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClaudeUsageSnapshot(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	t.Run("maps five_hour and seven_day windows", func(t *testing.T) {
		body := []byte(`{
			"five_hour": {"utilization": 12.5, "resets_at": "2023-11-14T23:00:00Z"},
			"seven_day": {"utilization": 40, "resets_at": "2023-11-19T12:34:56.789Z"},
			"seven_day_opus": {"utilization": 3, "resets_at": "2023-11-19T12:34:56Z"}
		}`)
		snap, err := parseClaudeUsageSnapshot(body, "max", now)
		require.NoError(t, err)
		assert.Equal(t, &SubscriptionUsageSnapshot{
			PlanType:        "max",
			PrimaryWindow:   &SubscriptionUsageWindow{UsedPercent: 12.5, ResetAt: 1700002800, LimitWindowSeconds: 5 * 3600},
			SecondaryWindow: &SubscriptionUsageWindow{UsedPercent: 40, ResetAt: 1700397296, LimitWindowSeconds: 7 * 24 * 3600},
			UpdatedAt:       now.Unix(),
		}, snap)
	})

	t.Run("null window and unparsable reset time", func(t *testing.T) {
		body := []byte(`{"five_hour": null, "seven_day": {"utilization": 250, "resets_at": "soon"}}`)
		snap, err := parseClaudeUsageSnapshot(body, "", now)
		require.NoError(t, err)
		assert.Nil(t, snap.PrimaryWindow)
		assert.Equal(t, &SubscriptionUsageWindow{UsedPercent: 100, LimitWindowSeconds: 7 * 24 * 3600}, snap.SecondaryWindow)
		assert.True(t, snap.LimitReached)
	})

	t.Run("rejects payload without windows", func(t *testing.T) {
		_, err := parseClaudeUsageSnapshot([]byte(`{"five_hour": null}`), "pro", now)
		require.Error(t, err)
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		_, err := parseClaudeUsageSnapshot([]byte(`<html>`), "pro", now)
		require.Error(t, err)
	})
}

func TestClaudeCLIVersionCache(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	t.Run("caches version for one hour", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			_, _ = w.Write([]byte(`{"version":" 2.1.293 "}`))
		}))
		defer srv.Close()

		var cache claudeCLIVersionCache
		v, err := cache.get(context.Background(), srv.Client(), srv.URL, now)
		require.NoError(t, err)
		assert.Equal(t, "2.1.293", v)

		v, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(30*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, "2.1.293", v)
		assert.EqualValues(t, 1, hits.Load())
	})

	t.Run("expired cache keeps stale value when refresh fails and re-caches it", func(t *testing.T) {
		var hits atomic.Int32
		var fail atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if fail.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"version":"2.1.293"}`))
		}))
		defer srv.Close()

		var cache claudeCLIVersionCache
		_, err := cache.get(context.Background(), srv.Client(), srv.URL, now)
		require.NoError(t, err)
		require.EqualValues(t, 1, hits.Load())

		fail.Store(true)
		v, err := cache.get(context.Background(), srv.Client(), srv.URL, now.Add(61*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, "2.1.293", v)
		require.EqualValues(t, 2, hits.Load())

		v, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(91*time.Minute))
		require.NoError(t, err)
		assert.Equal(t, "2.1.293", v)
		assert.EqualValues(t, 2, hits.Load())
	})

	t.Run("empty cache and failing upstream returns error", func(t *testing.T) {
		tests := []struct {
			name    string
			handler http.HandlerFunc
		}{
			{"blank version", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"version":"  "}`))
			}},
			{"http 500", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()

				var cache claudeCLIVersionCache
				_, err := cache.get(context.Background(), srv.Client(), srv.URL, now)
				require.Error(t, err)
			})
		}
	})

	t.Run("empty cache remembers failure for one hour", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		var cache claudeCLIVersionCache
		_, err := cache.get(context.Background(), srv.Client(), srv.URL, now)
		require.Error(t, err)
		_, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(30*time.Minute))
		require.Error(t, err)
		assert.EqualValues(t, 1, hits.Load())

		_, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(61*time.Minute))
		require.Error(t, err)
		assert.EqualValues(t, 2, hits.Load())
	})
}

func TestFetchClaudeOAuthUsage(t *testing.T) {
	t.Run("with cli version sends client headers and query", func(t *testing.T) {
		var got *http.Request
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Clone(r.Context())
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(`{"five_hour":null}`))
		}))
		defer srv.Close()

		status, body, err := fetchClaudeOAuthUsage(context.Background(), srv.Client(), srv.URL, "tok-placeholder", "2.1.293")
		require.NoError(t, err)
		assert.Equal(t, http.StatusTeapot, status)
		assert.Equal(t, `{"five_hour":null}`, string(body))

		require.NotNil(t, got)
		assert.Equal(t, "/api/oauth/usage", got.URL.Path)
		assert.Equal(t, "1", got.URL.Query().Get("cedar_ember"))
		assert.Equal(t, "1", got.URL.Query().Get("at_wall"))
		assert.Equal(t, "claude-cli/2.1.293 (external, cli)", got.Header.Get("User-Agent"))
		assert.Equal(t, "Bearer tok-placeholder", got.Header.Get("Authorization"))
		assert.Equal(t, "oauth-2025-04-20", got.Header.Get("anthropic-beta"))
		assert.Equal(t, "application/json", got.Header.Get("Accept"))
	})

	t.Run("without cli version omits query and claude-cli user agent", func(t *testing.T) {
		var got *http.Request
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Clone(r.Context())
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		status, _, err := fetchClaudeOAuthUsage(context.Background(), srv.Client(), srv.URL, "tok-placeholder", "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		require.NotNil(t, got)
		assert.Equal(t, "/api/oauth/usage", got.URL.Path)
		assert.False(t, got.URL.Query().Has("cedar_ember"))
		assert.False(t, got.URL.Query().Has("at_wall"))
		assert.NotContains(t, got.Header.Get("User-Agent"), "claude-cli/")
		assert.Equal(t, "Bearer tok-placeholder", got.Header.Get("Authorization"))
	})
}
