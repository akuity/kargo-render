package bookkeeper

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/akuityio/bookkeeper/internal/file"
	"github.com/stretchr/testify/require"
)

func TestWriteFiles(t *testing.T) {
	testYAMLChunk1 := []byte(`kind: Deployment
metadata:
  name: foobar
`)
	testYAMLChunk2 := []byte(`kind: Service
metadata:
  name: foobar
`)
	testYAMLBytes := bytes.Join(
		[][]byte{
			testYAMLChunk1,
			testYAMLChunk2,
		},
		[]byte("---\n"),
	)
	testDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)
	err = writeFiles(testDir, testYAMLBytes)
	require.NoError(t, err)
	filename := filepath.Join(testDir, "foobar-deployment.yaml")
	exists, err := file.Exists(filename)
	require.NoError(t, err)
	require.True(t, exists)
	fileBytes, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, testYAMLChunk1, fileBytes)
	filename = filepath.Join(testDir, "foobar-service.yaml")
	exists, err = file.Exists(filename)
	require.NoError(t, err)
	require.True(t, exists)
	fileBytes, err = os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, testYAMLChunk2, fileBytes)
}

func TestRmYAML(t *testing.T) {
	testDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)
	file1 := filepath.Join(testDir, "foo.yaml")
	file2 := filepath.Join(testDir, "bar.yaml")
	_, err = os.Create(file1)
	require.NoError(t, err)
	_, err = os.Create(file2)
	require.NoError(t, err)
	err = rmYAML(testDir)
	require.NoError(t, err)
	exists, err := file.Exists(file1)
	require.NoError(t, err)
	require.False(t, exists)
	exists, err = file.Exists(file2)
	require.NoError(t, err)
	require.False(t, exists)
}
