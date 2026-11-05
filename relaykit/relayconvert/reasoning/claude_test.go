package reasoning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeSupportedEfforts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		model     string
		want      []Effort
		wantKnown bool
	}{
		{"opus 4.5 可关闭、无 xhigh/max", "claude-opus-4-5-20251101", []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh}, true},
		{"opus 4.6 可关闭、支持 max", "claude-opus-4-6", []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh, EffortMax}, true},
		{"opus 4.7 可关闭、支持 xhigh 与 max", "claude-opus-4-7", []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"fable 5 不可关闭", "claude-fable-5", []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"opus 5.5 不可关闭", "claude-opus-5-5", []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"模型名大小写不敏感", "Claude-Opus-4-6", []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh, EffortMax}, true},
		{"sonnet 5.5 用 between_tools 关闭", "claude-sonnet-5-5", []Effort{EffortNone, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"sonnet 4.5 按预算换算、可关闭", "claude-sonnet-4-5", []Effort{EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"haiku 4.5 按预算换算、可关闭", "claude-haiku-4-5", []Effort{EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}, true},
		{"非 Claude 家族", "gpt-4o", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, known := ClaudeSupportedEfforts(tt.model)
			assert.Equal(t, tt.wantKnown, known)
			assert.Equal(t, tt.want, got)
		})
	}
}
