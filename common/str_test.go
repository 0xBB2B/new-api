package common

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalLogPreviewKeepsWholeUTF8Characters(t *testing.T) {
	oldDebug := DebugEnabled
	t.Cleanup(func() { DebugEnabled = oldDebug })
	DebugEnabled = false

	content := strings.Repeat("a", LocalLogContentLimit-1) + strings.Repeat("错", 10)
	preview := LocalLogPreview(content)

	kept, _, found := strings.Cut(preview, "... [truncated")
	require.True(t, found)
	assert.True(t, utf8.ValidString(kept))
	assert.Equal(t, strings.Repeat("a", LocalLogContentLimit-1), kept)
	assert.Contains(t, preview, "original_length=2077, limit=2048]")
}
