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
