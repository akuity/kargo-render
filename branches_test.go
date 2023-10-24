package render

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/akuity/kargo-render/internal/file"
)

func TestLoadBranchMetadata(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func() string
		assertions func(*branchMetadata, error)
	}{
		{
			name: "metadata does not exist",
			setup: func() string {
				repoDir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				return repoDir
			},
			assertions: func(md *branchMetadata, err error) {
				require.NoError(t, err)
				require.Nil(t, md)
			},
		},
		{
			name: "invalid YAML",
			setup: func() string {
				repoDir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				bkDir := filepath.Join(repoDir, ".kargo-render")
				err = os.Mkdir(bkDir, 0755)
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(bkDir, "metadata.yaml"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return repoDir
			},
			assertions: func(_ *branchMetadata, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "error unmarshaling branch metadata")
			},
		},
		{
			name: "valid YAML",
			setup: func() string {
				repoDir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				bkDir := filepath.Join(repoDir, ".kargo-render")
				err = os.Mkdir(bkDir, 0755)
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(bkDir, "metadata.yaml"),
					[]byte(""), // An empty file should actually be valid
					0600,
				)
				require.NoError(t, err)
				return repoDir
			},
			assertions: func(_ *branchMetadata, err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md, err := loadBranchMetadata(testCase.setup())
			testCase.assertions(md, err)
		})
	}
}

func TestWriteBranchMetadata(t *testing.T) {
	repoDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	err = writeBranchMetadata(
		branchMetadata{
			SourceCommit: "1234567",
		},
		repoDir,
	)
	require.NoError(t, err)
	exists, err :=
		file.Exists(filepath.Join(repoDir, ".kargo-render", "metadata.yaml"))
	require.NoError(t, err)
	require.True(t, exists)
}

func TestCleanCommitBranch(t *testing.T) {
	const subdirCount = 50
	const fileCount = 50
	// Create dummy repo dir
	dir, err := createDummyCommitBranchDir(subdirCount, fileCount)
	defer os.RemoveAll(dir)
	require.NoError(t, err)
	// Double-check the setup
	dirEntries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, dirEntries, subdirCount+fileCount+2)
	// Delete
	err = cleanCommitBranch(dir)
	require.NoError(t, err)
	// .git should not have been deleted
	_, err = os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err)
	// .kargo-render should not have been deleted
	_, err = os.Stat(filepath.Join(dir, ".kargo-render"))
	require.NoError(t, err)
	// Everything else should be deleted
	dirEntries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, dirEntries, 2)
}

func createDummyCommitBranchDir(dirCount, fileCount int) (string, error) {
	// Create a directory
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return dir, err
	}
	// Add a dummy .git/ subdir
	if err = os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		return dir, err
	}
	// Add a dummy .kargo-render/ subdir
	if err = os.Mkdir(filepath.Join(dir, ".kargo-render"), 0755); err != nil {
		return dir, err
	}
	// Add some other dummy dirs
	for i := 0; i < dirCount; i++ {
		if err = os.Mkdir(
			filepath.Join(dir, fmt.Sprintf("dir-%d", i)),
			0755,
		); err != nil {
			return dir, err
		}
	}
	// Add some dummy files
	for i := 0; i < fileCount; i++ {
		file, err := os.Create(filepath.Join(dir, fmt.Sprintf("file-%d", i)))
		if err != nil {
			return dir, err
		}
		if err = file.Close(); err != nil {
			return dir, err
		}
	}
	return dir, nil
}
