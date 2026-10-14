package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
			errText string
		}{
			{"blank version", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"version":"  "}`))
			}, "claude cli latest package has no version"},
			{"http 500", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}, "status=500"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()

				var cache claudeCLIVersionCache
				_, err := cache.get(context.Background(), srv.Client(), srv.URL, now)
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errText)
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
		require.ErrorContains(t, err, "status=500")
		_, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(30*time.Minute))
		require.ErrorContains(t, err, "failed recently")
		assert.EqualValues(t, 1, hits.Load())

		_, err = cache.get(context.Background(), srv.Client(), srv.URL, now.Add(61*time.Minute))
		require.ErrorContains(t, err, "status=500")
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

type fakeClaudeResetServer struct {
	*httptest.Server
	mu          sync.Mutex
	profileReqs []*http.Request
	resetReqs   []*http.Request
	resetBodies []string
}

func newFakeClaudeResetServer(t *testing.T, profileBody string, resetStatus int, resetBody string) *fakeClaudeResetServer {
	t.Helper()
	f := &fakeClaudeResetServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.URL.Path == "/api/oauth/profile" {
			f.profileReqs = append(f.profileReqs, r.Clone(r.Context()))
			_, _ = w.Write([]byte(profileBody))
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.resetReqs = append(f.resetReqs, r.Clone(r.Context()))
		f.resetBodies = append(f.resetBodies, string(b))
		w.WriteHeader(resetStatus)
		_, _ = w.Write([]byte(resetBody))
	}))
	t.Cleanup(f.Close)
	return f
}

const fakeClaudeProfile = `{"organization":{"uuid":"11111111-2222-3333-4444-555555555555"}}`

func TestRedeemClaudeLimitReset(t *testing.T) {
	cedar := ClaudeLimitResetRequest{Program: "cedar_ember", GrantID: "opus55-launch-promax-20260921", ResetsLeft: 1}

	t.Run("cedar_ember success sends profile and reset requests with client headers", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"reset"}`)

		res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", cedar)
		assert.True(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Reset succeeded", res.Message.Key)
		assert.Nil(t, res.Message.Params)

		require.Len(t, f.profileReqs, 1)
		require.Len(t, f.resetReqs, 1)
		reset := f.resetReqs[0]
		assert.Equal(t, http.MethodPost, reset.Method)
		assert.Equal(t, "/api/organizations/11111111-2222-3333-4444-555555555555/reset_rate_limits", reset.URL.Path)
		assert.JSONEq(t, `{"program":"cedar_ember","grant_id":"opus55-launch-promax-20260921","request_id":"opus55-launch-promax-20260921-u1"}`, f.resetBodies[0])
		for _, req := range []*http.Request{f.profileReqs[0], reset} {
			assert.Equal(t, "claude-cli/2.1.293 (external, cli)", req.Header.Get("User-Agent"))
			assert.Equal(t, "Bearer tok-placeholder", req.Header.Get("Authorization"))
			assert.Equal(t, "oauth-2025-04-20", req.Header.Get("anthropic-beta"))
		}
	})

	t.Run("same request twice produces identical request_id", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"reset"}`)

		redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", cedar)
		redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", cedar)

		require.Len(t, f.resetBodies, 2)
		assert.JSONEq(t, f.resetBodies[0], f.resetBodies[1])
		assert.Contains(t, f.resetBodies[0], `"request_id":"opus55-launch-promax-20260921-u1"`)
	})

	t.Run("juniper_tide body only has program and not_limited fails", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"not_limited"}`)

		res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", ClaudeLimitResetRequest{Program: "juniper_tide"})
		assert.False(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Not currently rate limited, no reset needed", res.Message.Key)
		assert.Nil(t, res.Message.Params)
		require.Len(t, f.resetBodies, 1)
		assert.JSONEq(t, `{"program":"juniper_tide"}`, f.resetBodies[0])
	})

	t.Run("upstream result and status map to messages", func(t *testing.T) {
		tests := []struct {
			name     string
			status   int
			body     string
			key      string
			wantArgs map[string]any
		}{
			{"already_used", http.StatusOK, `{"result":"already_used"}`, "This reset has already been used", nil},
			{"cooldown", http.StatusOK, `{"result":"cooldown"}`, "On cooldown, please try again later", nil},
			{"ineligible", http.StatusOK, `{"result":"ineligible"}`, "Account is not eligible", nil},
			{"unavailable", http.StatusOK, `{"result":"unavailable"}`, "Upstream temporarily unavailable", nil},
			{"unknown result", http.StatusOK, `{"result":"something_new"}`, "Upstream returned an unknown result", nil},
			{"non json body", http.StatusOK, `not json`, "Upstream returned an unknown result", nil},
			{"http 429", http.StatusTooManyRequests, `{}`, "Too many requests, please try again later", nil},
			{"http 401", http.StatusUnauthorized, `{}`, "Invalid credentials or insufficient permissions", nil},
			{"http 500", http.StatusInternalServerError, `{}`, "Upstream returned HTTP {{status}}", map[string]any{"status": 500}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := newFakeClaudeResetServer(t, fakeClaudeProfile, tt.status, tt.body)
				res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", cedar)
				assert.False(t, res.Success)
				require.NotNil(t, res.Message)
				assert.Equal(t, tt.key, res.Message.Key)
				assert.Equal(t, tt.wantArgs, res.Message.Params)
				assert.Equal(t, tt.status, res.UpstreamStatus)
			})
		}
	})

	t.Run("invalid input is rejected without calling reset", func(t *testing.T) {
		tests := []struct {
			name string
			req  ClaudeLimitResetRequest
			key  string
		}{
			{"unknown program", ClaudeLimitResetRequest{Program: "foo"}, "Unsupported reset type"},
			{"grant_id path traversal", ClaudeLimitResetRequest{Program: "cedar_ember", GrantID: "../x", ResetsLeft: 1}, "Invalid reset parameters"},
			{"grant_id 41 chars", ClaudeLimitResetRequest{Program: "cedar_ember", GrantID: strings.Repeat("a", 41), ResetsLeft: 1}, "Invalid reset parameters"},
			{"resets_left 0", ClaudeLimitResetRequest{Program: "cedar_ember", GrantID: "g1", ResetsLeft: 0}, "Invalid reset parameters"},
			{"resets_left 101", ClaudeLimitResetRequest{Program: "cedar_ember", GrantID: "g1", ResetsLeft: 101}, "Invalid reset parameters"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"reset"}`)
				res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", tt.req)
				assert.False(t, res.Success)
				require.NotNil(t, res.Message)
				assert.Equal(t, tt.key, res.Message.Key)
				assert.Nil(t, res.Message.Params)
				assert.Empty(t, f.resetReqs)
			})
		}
	})

	t.Run("malformed organization uuid is rejected without calling reset", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, `{"organization":{"uuid":"not-a-uuid"}}`, http.StatusOK, `{"result":"reset"}`)

		res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "2.1.293", cedar)
		assert.False(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Failed to get account organization info", res.Message.Key)
		assert.Nil(t, res.Message.Params)
		assert.Empty(t, f.resetReqs)
	})

	t.Run("empty cli version sends no upstream requests", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"reset"}`)

		res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "", cedar)
		assert.False(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Unable to get the latest Claude Code version, cannot perform reset", res.Message.Key)
		assert.Nil(t, res.Message.Params)
		assert.Empty(t, f.profileReqs)
		assert.Empty(t, f.resetReqs)
	})

	t.Run("invalid program is reported before missing cli version", func(t *testing.T) {
		f := newFakeClaudeResetServer(t, fakeClaudeProfile, http.StatusOK, `{"result":"reset"}`)

		res := redeemClaudeLimitReset(context.Background(), f.Client(), f.URL, "tok-placeholder", "", ClaudeLimitResetRequest{Program: "foo"})
		assert.False(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Unsupported reset type", res.Message.Key)
		assert.Nil(t, res.Message.Params)
		assert.Empty(t, f.profileReqs)
		assert.Empty(t, f.resetReqs)
	})

	t.Run("lost reset response asks to refresh before retrying", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/api/oauth/profile") {
				_, _ = w.Write([]byte(fakeClaudeProfile))
				return
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			_ = conn.Close()
		}))
		defer srv.Close()

		res := redeemClaudeLimitReset(context.Background(), srv.Client(), srv.URL, "tok-placeholder", "2.1.293", cedar)
		assert.False(t, res.Success)
		require.NotNil(t, res.Message)
		assert.Equal(t, "Reset outcome unknown, refresh usage to confirm before retrying", res.Message.Key)
		assert.Nil(t, res.Message.Params)
	})
}
