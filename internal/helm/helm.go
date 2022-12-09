package helm

import (
	"os/exec"
	"path/filepath"

	libExec "github.com/akuityio/bookkeeper/internal/exec"
)

// TODO: Document this
func PreRender(
	repoRoot string,
	targetBranch string,
	cfg *Config,
) ([]byte, error) {
	return libExec.Exec(buildPreRenderCmd(repoRoot, targetBranch, cfg))
}

func buildPreRenderCmd(
	repoRoot string,
	targetBranch string,
	cfg *Config,
) *exec.Cmd {
	var chartPath string
	if cfg.ChartPath != "" {
		chartPath = cfg.ChartPath
	} else {
		chartPath = "base"
	}
	cmdArgs := []string{"template", cfg.ReleaseName, chartPath}
	if len(cfg.ValuesPaths) > 0 {
		for _, valuePath := range cfg.ValuesPaths {
			cmdArgs = append(cmdArgs, "--values", valuePath)
		}
	} else {
		cmdArgs = append(
			cmdArgs,
			"--values",
			filepath.Join(targetBranch, "values.yaml"),
		)
	}
	cmd := exec.Command("helm", cmdArgs...)
	cmd.Dir = repoRoot
	return cmd
}
