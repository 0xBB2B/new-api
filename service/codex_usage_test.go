package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexUsageSnapshot(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	t.Run("keeps both windows and plan", func(t *testing.T) {
		body := []byte(`{
			"plan_type": "plus",
			"rate_limit": {
				"allowed": true,
				"limit_reached": false,
				"primary_window": {"used_percent": 12.5, "reset_at": 1700003600, "reset_after_seconds": 3600, "limit_window_seconds": 18000},
				"secondary_window": {"used_percent": 40, "reset_at": 1700400000, "limit_window_seconds": 604800}
			}
		}`)
		snap, err := parseCodexUsageSnapshot(body, now)
		require.NoError(t, err)
		assert.Equal(t, &CodexUsageSnapshot{
			PlanType:        "plus",
			PrimaryWindow:   &CodexUsageWindow{UsedPercent: 12.5, ResetAt: 1700003600, LimitWindowSeconds: 18000},
			SecondaryWindow: &CodexUsageWindow{UsedPercent: 40, ResetAt: 1700400000, LimitWindowSeconds: 604800},
			UpdatedAt:       now.Unix(),
		}, snap)
	})

	t.Run("single window and limit reached", func(t *testing.T) {
		body := []byte(`{"plan_type":"free","rate_limit":{"limit_reached":true,"primary_window":{"used_percent":100,"limit_window_seconds":604800}}}`)
		snap, err := parseCodexUsageSnapshot(body, now)
		require.NoError(t, err)
		assert.True(t, snap.LimitReached)
		assert.Equal(t, &CodexUsageWindow{UsedPercent: 100, LimitWindowSeconds: 604800}, snap.PrimaryWindow)
		assert.Nil(t, snap.SecondaryWindow)
	})

	t.Run("clamps out-of-range percent", func(t *testing.T) {
		body := []byte(`{"rate_limit":{"primary_window":{"used_percent":180},"secondary_window":{"used_percent":-3}}}`)
		snap, err := parseCodexUsageSnapshot(body, now)
		require.NoError(t, err)
		assert.Equal(t, float64(100), snap.PrimaryWindow.UsedPercent)
		assert.Equal(t, float64(0), snap.SecondaryWindow.UsedPercent)
	})

	t.Run("rejects payload without windows", func(t *testing.T) {
		_, err := parseCodexUsageSnapshot([]byte(`{"plan_type":"plus","rate_limit":{}}`), now)
		require.Error(t, err)
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		_, err := parseCodexUsageSnapshot([]byte(`not json`), now)
		require.Error(t, err)
	})
}
