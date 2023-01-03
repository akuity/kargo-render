package kustomize

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigExpand(t *testing.T) {
	const val = "foo"
	testCfg := Config{
		Path: "${0}",
	}
	cfg := testCfg.Expand([]string{val})
	require.Equal(t, cfg.Path, val)
}
