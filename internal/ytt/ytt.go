package ytt

import (
	"context"
	"os/exec"

	libExec "github.com/akuityio/bookkeeper/internal/exec"
)

// Render shells out to the ytt binary to render the provided paths into plain
// YAML manifests. Unlike in the case of Kustomize and Helm, this is not done
// with the help of the Argo CD repo server, since that does not yet support
// ytt.
func Render(_ context.Context, paths []string) ([]byte, error) {
	cmdArgs := make([]string, len(paths)*2)
	for i, path := range paths {
		cmdArgs[i*2] = "--file"
		cmdArgs[i*2+1] = path
	}
	return libExec.Exec(exec.Command("ytt", cmdArgs...))
}
