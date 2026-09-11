package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildSubscriptionUsageOtherInfo(t *testing.T, updatedAt int64) string {
	t.Helper()
	data, err := common.Marshal(map[string]any{
		subscriptionUsageOtherInfoKey: SubscriptionUsageSnapshot{UpdatedAt: updatedAt},
	})
	require.NoError(t, err)
	return string(data)
}

func TestShouldPollSubscriptionUsage(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		otherInfo   func(t *testing.T) string
		lastAttempt time.Time
		want        bool
	}{
		{
			name:        "empty other_info polls",
			otherInfo:   func(t *testing.T) string { return "" },
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name:        "other_info present without snapshot key polls",
			otherInfo:   func(t *testing.T) string { return `{"foo":"bar"}` },
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name: "snapshot key present without updated_at polls",
			otherInfo: func(t *testing.T) string {
				return `{"subscription_usage":{"plan_type":"pro"}}`
			},
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name: "snapshot updated_at zero polls",
			otherInfo: func(t *testing.T) string {
				return `{"subscription_usage":{"updated_at":0}}`
			},
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name: "snapshot exactly one hour old polls",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-time.Hour).Unix())
			},
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name: "snapshot one second under one hour skips",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-time.Hour+time.Second).Unix())
			},
			lastAttempt: time.Time{},
			want:        false,
		},
		{
			name: "snapshot well under one hour skips",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-30*time.Minute).Unix())
			},
			lastAttempt: time.Time{},
			want:        false,
		},
		{
			name: "snapshot stale but process attempted recently skips",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-2*time.Hour).Unix())
			},
			lastAttempt: now.Add(-30 * time.Minute),
			want:        false,
		},
		{
			name: "snapshot stale and last attempt exactly one hour ago polls",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-2*time.Hour).Unix())
			},
			lastAttempt: now.Add(-time.Hour),
			want:        true,
		},
		{
			name: "snapshot stale and last attempt just under one hour ago skips",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-2*time.Hour).Unix())
			},
			lastAttempt: now.Add(-time.Hour + time.Second),
			want:        false,
		},
		{
			name: "restart with fresh snapshot and zero last attempt skips",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-30*time.Minute).Unix())
			},
			lastAttempt: time.Time{},
			want:        false,
		},
		{
			name: "restart with stale snapshot and zero last attempt polls",
			otherInfo: func(t *testing.T) string {
				return buildSubscriptionUsageOtherInfo(t, now.Add(-2*time.Hour).Unix())
			},
			lastAttempt: time.Time{},
			want:        true,
		},
		{
			name:        "malformed other_info json polls",
			otherInfo:   func(t *testing.T) string { return `{invalid json` },
			lastAttempt: time.Time{},
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shouldPollSubscriptionUsage(tc.otherInfo(t), tc.lastAttempt, now)
			assert.Equal(t, tc.want, got)
		})
	}
}
