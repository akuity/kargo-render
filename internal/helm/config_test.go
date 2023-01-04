package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigExpand(t *testing.T) {
	const val = "foo"
	testCfg := Config{
		ChartPath:   "${0}",
		ValuesPaths: []string{"${0}", "${0}"},
	}
	cfg := testCfg.Expand([]string{val})
	require.Equal(t, cfg.ChartPath, val)
	require.Equal(t, cfg.ValuesPaths, []string{val, val})
	// Check that original testCfg.ValuesPaths are untouched.
	// Slice references are pointers, hence the extra care.
	require.Equal(t, []string{"${0}", "${0}"}, testCfg.ValuesPaths)
}
