package render

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/akuity/kargo-render/internal/file"
)

func TestNewService(t *testing.T) {
	s := NewService(nil)
	svc, ok := s.(*service)
	require.True(t, ok)
	require.NotNil(t, svc.logger)
	require.NotNil(t, svc.renderFn)
}

func TestWriteAppManifests(t *testing.T) {
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
	testDir := t.TempDir()
	err := writeManifests(testDir, testYAMLBytes)
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
