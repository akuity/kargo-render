package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRepoConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func() string
		assertions func(*testing.T, error)
	}{
		{
			name: "invalid JSON",
			setup: func() string {
				dir := t.TempDir()
				err := os.WriteFile(
					filepath.Join(dir, "kargo-render.json"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error normalizing and validating Kargo Render configuration",
				)
			},
		},
		{
			name: "invalid YAML",
			setup: func() string {
				dir := t.TempDir()
				err := os.WriteFile(
					filepath.Join(dir, "kargo-render.yaml"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error normalizing and validating Kargo Render configuration",
				)
			},
		},
		{
			name: "valid JSON",
			setup: func() string {
				dir := t.TempDir()
				err := os.WriteFile(
					filepath.Join(dir, "kargo-render.json"),
					[]byte(`{"configVersion": "v1alpha1"}`),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "valid YAML",
			setup: func() string {
				dir := t.TempDir()
				err := os.WriteFile(
					filepath.Join(dir, "kargo-render.yaml"),
					[]byte("configVersion: v1alpha1"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := loadRepoConfig(testCase.setup())
			testCase.assertions(t, err)
		})
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	testCases := []struct {
		name       string
		config     []byte
		assertions func(*testing.T, error)
	}{
		{
			name:   "invalid JSON",
			config: []byte("{}"),
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "invalid YAML",
			config: []byte(""),
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "valid JSON",
			config: []byte(`{"configVersion": "v1alpha1"}`),
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "valid YAML",
			config: []byte("configVersion: v1alpha1"),
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "valid kustomize",
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			config: []byte(`configVersion: v1alpha1
branchConfigs:
  - name: env/prod
    appConfigs:
      my-proj:
        configManagement:
          path: env/prod/my-proj
          kustomize:
            buildOptions: "--load-restrictor LoadRestrictionsNone"
        outputPath: prod/my-proj
        combineManifests: true`),
		},
		{
			name: "valid helm",
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			config: []byte(`configVersion: v1alpha1
branchConfigs:
  - name: env/prod
    appConfigs:
      my-proj:
        configManagement:
          path: env/prod/my-proj
          helm:
            namespace: my-namespace
        outputPath: prod/my-proj
        combineManifests: true`),
		},
		{
			name: "valid no config management tool",
			assertions: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			config: []byte(`configVersion: v1alpha1
branchConfigs:
  - name: env/prod
    appConfigs:
      my-proj:
        configManagement:
          path: env/prod/my-proj
        outputPath: prod/my-proj
        combineManifests: true`),
		},
		{
			name: "invalid property",
			assertions: func(t *testing.T, err error) {
				require.Error(t, err)
			},
			config: []byte(`configVersion: v1alpha1
branchConfigs:
  - name: env/prod
    appConfigs:
      my-proj:
        configManagement:
          path: env/prod/my-proj
          unknown:
            hello: world
        outputPath: prod/my-proj
        combineManifests: true`),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configBytes, err := normalizeAndValidate(testCase.config)
			testCase.assertions(t, err)
			// For any validation that doesn't fail, the bytes returned should be
			// JSON we can unmarshal...
			if err == nil {
				cfg := repoConfig{}
				err = json.Unmarshal(configBytes, &cfg)
				require.NoError(t, err)
			}
		})
	}
}
