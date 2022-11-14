package helm

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akuityio/bookkeeper/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildPreRenderCmd(t *testing.T) {
	const testRepoRoot = "/tmp/foo"
	const testReleaseName = "bar"
	const testTargetBranchName = "env/dev"
	testCases := []struct {
		name       string
		cfg        *config.HelmConfig
		assertions func(*exec.Cmd)
	}{
		{
			name: "chartPath not specified, valuesPaths empty",
			cfg: &config.HelmConfig{
				ReleaseName: testReleaseName,
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command( // nolint: gosec
					"helm",
					"template",
					testReleaseName,
					"base",
					"--values",
					filepath.Join(testTargetBranchName, "values.yaml"),
				)
				expectedCmd.Dir = testRepoRoot
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "chartPath specified, valuesPaths empty",
			cfg: &config.HelmConfig{
				ReleaseName: testReleaseName,
				ChartPath:   "my-chart-path",
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command( // nolint: gosec
					"helm",
					"template",
					testReleaseName,
					"my-chart-path",
					"--values",
					filepath.Join(testTargetBranchName, "values.yaml"),
				)
				expectedCmd.Dir = testRepoRoot
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "chartPath not specified, valuesPaths provided",
			cfg: &config.HelmConfig{
				ReleaseName: testReleaseName,
				ValuesPaths: []string{"abc/values.yaml", "xyz/values.yaml"},
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command(
					"helm",
					"template",
					testReleaseName,
					"base",
					"--values",
					"abc/values.yaml",
					"--values",
					"xyz/values.yaml",
				)
				expectedCmd.Dir = testRepoRoot
				require.Equal(t, expectedCmd, cmd)
			},
		},
		{
			name: "chartPath specified, valuesPaths provided",
			cfg: &config.HelmConfig{
				ReleaseName: testReleaseName,
				ChartPath:   "my-chart-path",
				ValuesPaths: []string{"abc/values.yaml", "xyz/values.yaml"},
			},
			assertions: func(cmd *exec.Cmd) {
				expectedCmd := exec.Command(
					"helm",
					"template",
					testReleaseName,
					"my-chart-path",
					"--values",
					"abc/values.yaml",
					"--values",
					"xyz/values.yaml",
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
