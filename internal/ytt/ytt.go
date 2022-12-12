package ytt

import (
	"context"
	"os/exec"

	libExec "github.com/akuityio/bookkeeper/internal/exec"
)

// TODO: Document this
func Render(_ context.Context, paths []string) ([]byte, error) {
	cmdArgs := make([]string, len(paths)*2)
	for i, path := range paths {
		cmdArgs[i*2] = "--file"
		cmdArgs[i*2+1] = path
	}
	return libExec.Exec(exec.Command("ytt", cmdArgs...))
}
