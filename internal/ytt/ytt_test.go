package ytt

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPreRenderCmd(t *testing.T) {
	const testRepoRoot = "/tmp/foo"
	const testTargetBranchName = "env/dev"
	testCases := []struct {
		name       string
		cfg        *Config
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
			cfg:  &Config{},
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
			cfg: &Config{
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
