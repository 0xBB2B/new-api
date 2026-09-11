package service

import (
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
