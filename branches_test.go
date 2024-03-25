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
		assertions func(*testing.T, *branchMetadata, error)
	}{
		{
			name: "metadata does not exist",
			setup: func() string {
				return t.TempDir()
			},
			assertions: func(t *testing.T, md *branchMetadata, err error) {
				require.NoError(t, err)
				require.Nil(t, md)
			},
		},
		{
			name: "invalid YAML",
			setup: func() string {
				repoDir := t.TempDir()
				bkDir := filepath.Join(repoDir, ".kargo-render")
				err := os.Mkdir(bkDir, 0755)
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(bkDir, "metadata.yaml"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return repoDir
			},
			assertions: func(t *testing.T, _ *branchMetadata, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "error unmarshaling branch metadata")
			},
		},
		{
			name: "valid YAML",
			setup: func() string {
				repoDir := t.TempDir()
				bkDir := filepath.Join(repoDir, ".kargo-render")
				err := os.Mkdir(bkDir, 0755)
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(bkDir, "metadata.yaml"),
					[]byte(""), // An empty file should actually be valid
					0600,
				)
				require.NoError(t, err)
				return repoDir
			},
			assertions: func(t *testing.T, _ *branchMetadata, err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			md, err := loadBranchMetadata(testCase.setup())
			testCase.assertions(t, md, err)
		})
	}
}

func TestWriteBranchMetadata(t *testing.T) {
	repoDir := t.TempDir()
	err := writeBranchMetadata(
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
	dir, err := createDummyCommitBranchDir(t, subdirCount, fileCount)
	require.NoError(t, err)
	// Double-check the setup
	dirEntries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, dirEntries, subdirCount+fileCount+2)
	// Delete
	err = cleanCommitBranch(dir, []string{})
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

func TestCopyBranchContents(t *testing.T) {
	const subdirCount = 50
	const fileCount = 50
	// Create dummy repo srcDir
	srcDir, err := createDummyCommitBranchDir(t, subdirCount, fileCount)
	require.NoError(t, err)
	// Double-check the setup
	dirEntries, err := os.ReadDir(srcDir)
	require.NoError(t, err)
	require.Len(t, dirEntries, subdirCount+fileCount+2)
	dstDir := filepath.Join(t.TempDir(), "dst")
	// Copy
	err = copyBranchContents(srcDir, dstDir)
	require.NoError(t, err)
	// .git should not have been included
	_, err = os.Stat(filepath.Join(dstDir, ".git"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	// Everything else should have been copied
	dirEntries, err = os.ReadDir(dstDir)
	require.NoError(t, err)
	require.Len(t, dirEntries, subdirCount+fileCount+1)
}

func TestNormalizePreservedPaths(t *testing.T) {
	preservedPaths := []string{
		"foo/bar",
		"bat/baz/",
	}
	normalizedPreservedPaths :=
		normalizePreservedPaths("fake-work-dir", preservedPaths)
	require.Equal(
		t,
		[]string{
			filepath.Join("fake-work-dir", "foo", "bar"),
			filepath.Join("fake-work-dir", "bat", "baz"),
		},
		normalizedPreservedPaths,
	)
}

func TestCleanDir(t *testing.T) {
	dir := t.TempDir()

	// This is what the test directory structure will look like:
	// .
	// ├── foo            preserved directly
	// │   └── foo.txt    preserved because foo is
	// ├── bar            preserved because bar/bar.txt is
	// │   └── bar.txt    preserved directly
	// ├── baz            deleted because empty
	// │   └── baz.txt    deleted
	// └── keep.txt       preserved directly

	// Create the test directory structure
	fooDir := filepath.Join(dir, "foo")
	err := os.Mkdir(fooDir, 0755)
	require.NoError(t, err)
	fooFile := filepath.Join(fooDir, "foo.txt")
	err = os.WriteFile(fooFile, []byte("foo"), 0600)
	require.NoError(t, err)

	barDir := filepath.Join(dir, "bar")
	err = os.Mkdir(barDir, 0755)
	require.NoError(t, err)
	barFile := filepath.Join(barDir, "bar.txt")
	err = os.WriteFile(barFile, []byte("bar"), 0600)
	require.NoError(t, err)

	bazDir := filepath.Join(dir, "baz")
	err = os.Mkdir(bazDir, 0755)
	require.NoError(t, err)
	bazFile := filepath.Join(bazDir, "baz.txt")
	err = os.WriteFile(bazFile, []byte("baz"), 0600)
	require.NoError(t, err)

	keepFile := filepath.Join(dir, "keep.txt")
	err = os.WriteFile(keepFile, []byte("keep"), 0600)
	require.NoError(t, err)

	preservedPaths := []string{
		fooDir,
		barFile,
		keepFile,
	}

	isEmpty, err := cleanDir(dir, preservedPaths)
	require.NoError(t, err)
	require.False(t, isEmpty)

	// Validate what was deleted and what wasn't

	// All of foo/ remains
	_, err = os.Stat(fooDir)
	require.NoError(t, err)
	_, err = os.Stat(fooFile)
	require.NoError(t, err)

	// All of bar/ remains
	_, err = os.Stat(barDir)
	require.NoError(t, err)
	_, err = os.Stat(barFile)
	require.NoError(t, err)

	// All of baz/ is gone
	_, err = os.Stat(bazDir)
	require.True(t, os.IsNotExist(err))

	// keep.txt remains
	_, err = os.Stat(keepFile)
	require.NoError(t, err)
}

func TestIsPathPreserved(t *testing.T) {
	preservedPaths := []string{
		"/foo/bar",
		"/foo/bat",
	}
	require.True(t, isPathPreserved("/foo/bar", preservedPaths))
	require.True(t, isPathPreserved("/foo/bat", preservedPaths))
	require.False(t, isPathPreserved("/foo/baz", preservedPaths))
}

func createDummyCommitBranchDir(t *testing.T, dirCount, fileCount int) (string, error) {
	// Create a directory
	dir := t.TempDir()
	// Add a dummy .git/ subdir
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		return dir, err
	}
	// Add a dummy .kargo-render/ subdir
	if err := os.Mkdir(filepath.Join(dir, ".kargo-render"), 0755); err != nil {
		return dir, err
	}
	// Add some other dummy dirs
	for i := 0; i < dirCount; i++ {
		if err := os.Mkdir(
			filepath.Join(dir, fmt.Sprintf("dir-%d", i)),
			0755,
		); err != nil {
			return dir, err
		}
	}
	// Add some dummy files
	for i := 0; i < fileCount; i++ {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("file-%d", i)))
		if err != nil {
			return dir, err
		}
		if err = f.Close(); err != nil {
			return dir, err
		}
	}
	return dir, nil
}
