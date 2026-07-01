package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iximiuz/labctl/api"
)

func TestValidateKinds(t *testing.T) {
	require.NoError(t, validateKinds(nil))
	require.NoError(t, validateKinds([]string{"challenge", "tutorial", "skill-path"}))

	err := validateKinds([]string{"challenge", "bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hello", truncate("hello", 5))
	assert.Equal(t, "hell…", truncate("hello world", 5))
	// Multi-byte runes must be counted as single characters, not bytes.
	assert.Equal(t, "café", truncate("café", 4))
}

func TestOneLine(t *testing.T) {
	assert.Equal(t, "a b c", oneLine("a\n  b\t c"))
	assert.Equal(t, "hello world", oneLine("  hello   world  "))
}

func TestPlural(t *testing.T) {
	assert.Equal(t, "", plural(1))
	assert.Equal(t, "s", plural(0))
	assert.Equal(t, "s", plural(2))
}

func TestKindBadgePlain(t *testing.T) {
	s := newStyler(false)
	assert.Equal(t, "[CHALLENGE]", s.kindBadge("challenge"))
	assert.Equal(t, "[VENDOR]", s.kindBadge("vendor"))
}

func TestMetaLinePlain(t *testing.T) {
	s := newStyler(false)

	got := metaLine(s, api.SearchItem{
		Difficulty:      "medium",
		Categories:      []string{"linux", "containers"},
		AttemptCount:    1234,
		CompletionCount: 42,
	})
	assert.Equal(t, "medium  ·  linux, containers  ·  1,234 attempts  ·  42 completed", got)

	assert.Equal(t, "", metaLine(s, api.SearchItem{}))
}
