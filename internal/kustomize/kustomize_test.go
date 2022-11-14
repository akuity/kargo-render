package kustomize

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPreRenderCmd(t *testing.T) {
	const testRepoRoot = "/tmp/foo"
	const testTargetBranchName = "env/dev"
	testCases := []struct {
		name       string
		cfg        *config.KustomizeConfig
		assertions func(*exec.Cmd)
	}{
		{
			name: "nil config",
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command("kustomize", "build")
				expectedCmd.Dir = filepath.Join(testRepoRoot, testTargetBranchName)
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "path not specified",
			cfg:  &config.KustomizeConfig{},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command("kustomize", "build")
				expectedCmd.Dir = filepath.Join(testRepoRoot, testTargetBranchName)
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "path specified",
			cfg: &config.KustomizeConfig{
				Path: "my-path",
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command("kustomize", "build")
				expectedCmd.Dir = filepath.Join(testRepoRoot, "my-path")
				require.Equal(t, expectedCmd, cmd)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cmd := buildPreRenderCmd(testRepoRoot, testTargetBranchName, testCase.cfg)
			testCase.assertions(cmd)
		})
	}
}
