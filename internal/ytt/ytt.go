package ytt

import (
	"os/exec"

	"github.com/pkg/errors"
)

// TODO: Document this
func PreRender(
	repoRoot string,
	targetBranch string,
	cfg *Config,
) ([]byte, error) {
	cmd := buildPreRenderCmd(repoRoot, targetBranch, cfg)
	yamlBytes, err := cmd.Output()
	return yamlBytes, errors.Wrapf(
		err,
		"error running `%s`",
		cmd.String(),
	)
}

func buildPreRenderCmd(
	repoRoot string,
	targetBranch string,
	cfg *Config,
) *exec.Cmd {
	if cfg == nil {
		cfg = &Config{}
	}
	var cmdArgs []string
	if len(cfg.Paths) > 0 {
		cmdArgs = make([]string, len(cfg.Paths)*2)
		for i, path := range cfg.Paths {
			cmdArgs[i*2] = "--file"
			cmdArgs[i*2+1] = path
		}
	} else {
		cmdArgs = []string{"--file", "base", "--file", targetBranch}
	}
	cmd := exec.Command("ytt", cmdArgs...)
	cmd.Dir = repoRoot
	return cmd
}
