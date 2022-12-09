package helm

import (
	"context"
	"os/exec"

	libExec "github.com/akuityio/bookkeeper/internal/exec"
)

// TODO: Document this
// TODO: Use repo server to do this
func Render(
	_ context.Context,
	releaseName string,
	chartPath string,
	valuesPaths []string,
) ([]byte, error) {
	cmdArgs := []string{"template", releaseName, chartPath}
	for _, valuesPath := range valuesPaths {
		cmdArgs = append(cmdArgs, "--values", valuesPath)
	}
	cmd := exec.Command("helm", cmdArgs...)
	return libExec.Exec(cmd)
}
