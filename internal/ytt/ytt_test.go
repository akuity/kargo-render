package ytt

import (
	"os/exec"
	"testing"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPreRenderCmd(t *testing.T) {
	const testRepoRoot = "/tmp/foo"
	const testTargetBranchName = "env/dev"
	testCases := []struct {
		name       string
		cfg        *config.YttConfig
		assertions func(*exec.Cmd)
	}{
		{
			name: "nil config",
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command(
					"ytt",
					"--file",
					"base",
					"--file",
					testTargetBranchName,
				)
				expectedCmd.Dir = testRepoRoot
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "paths empty",
			cfg:  &config.YttConfig{},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command(
					"ytt",
					"--file",
					"base",
					"--file",
					testTargetBranchName,
				)
				expectedCmd.Dir = testRepoRoot
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "paths specified",
			cfg: &config.YttConfig{
				Paths: []string{"abc", "xyz"},
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command(
					"ytt",
					"--file",
					"abc",
					"--file",
					"xyz",
				)
				expectedCmd.Dir = testRepoRoot
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
