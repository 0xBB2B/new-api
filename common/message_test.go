package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageErrorJoinsStringSliceParams(t *testing.T) {
	message := NewMessage("{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}", map[string]any{
		"path": "architecture.input_modalities", "value": "video", "allowed": []string{"text", "image", "file", "audio"},
	})

	assert.Equal(t,
		"architecture.input_modalities contains an unknown value video. Allowed values: text, image, file, audio",
		message.Error())
}
