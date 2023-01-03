package ytt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigExpand(t *testing.T) {
	const val = "foo"
	testCfg := Config{
		Paths: []string{"${0}", "${0}"},
	}
	cfg := testCfg.Expand([]string{val})
	require.Equal(t, cfg.Paths, []string{val, val})
	// Check that original testCfg.Paths are untouched.
	// Slice references are pointers, hence the extra care.
	require.Equal(t, []string{"${0}", "${0}"}, testCfg.Paths)
}
