package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/akuity/kargo-render/internal/helm"
	"github.com/akuity/kargo-render/internal/kustomize"
	"github.com/akuity/kargo-render/internal/ytt"
)

func TestLoadRepoConfig(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func() string
		assertions func(error)
	}{
		{
			name: "invalid JSON",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "kargo-render.json"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
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
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "kargo-render.yaml"),
					[]byte("bogus"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
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
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "kargo-render.json"),
					[]byte(`{"configVersion": "v1alpha1"}`),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "valid YAML",
			setup: func() string {
				dir, err := os.MkdirTemp("", "")
				require.NoError(t, err)
				err = os.WriteFile(
					filepath.Join(dir, "kargo-render.yaml"),
					[]byte("configVersion: v1alpha1"),
					0600,
				)
				require.NoError(t, err)
				return dir
			},
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := loadRepoConfig(testCase.setup())
			testCase.assertions(err)
		})
	}
}

func TestNormalizeAndValidate(t *testing.T) {
	testCases := []struct {
		name       string
		config     []byte
		assertions func(error)
	}{
		{
			name:   "invalid JSON",
			config: []byte("{}"),
			assertions: func(err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "invalid YAML",
			config: []byte(""),
			assertions: func(err error) {
				require.Error(t, err)
			},
		},
		{
			name:   "valid JSON",
			config: []byte(`{"configVersion": "v1alpha1"}`),
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
		{
			name:   "valid YAML",
			config: []byte("configVersion: v1alpha1"),
			assertions: func(err error) {
				require.NoError(t, err)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configBytes, err := normalizeAndValidate(testCase.config)
			testCase.assertions(err)
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

func TestExpandBranchConfig(t *testing.T) {
	const val = "foo"
	testCfg := branchConfig{
		AppConfigs: map[string]appConfig{
			"my-kustomize-app": {
				ConfigManagement: configManagementConfig{
					Kustomize: &kustomize.Config{
						Path: "${0}",
					},
				},
				OutputPath: "${0}",
			},
			"my-helm-app": {
				ConfigManagement: configManagementConfig{
					Helm: &helm.Config{
						ChartPath:   "${0}",
						ValuesPaths: []string{"${0}", "${0}"},
					},
				},
				OutputPath: "${0}",
			},
			"my-ytt-app": {
				ConfigManagement: configManagementConfig{
					Ytt: &ytt.Config{
						Paths: []string{"${0}", "${0}"},
					},
				},
				OutputPath: "${0}",
			},
		},
	}
	cfg := testCfg.expand([]string{val})
	require.Equal(
		t,
		val,
		cfg.AppConfigs["my-kustomize-app"].ConfigManagement.Kustomize.Path,
	)
	require.Equal(
		t,
		val,
		cfg.AppConfigs["my-kustomize-app"].OutputPath,
	)
	require.Equal(
		t,
		val,
		cfg.AppConfigs["my-helm-app"].ConfigManagement.Helm.ChartPath,
	)
	require.Equal(
		t,
		[]string{val, val},
		cfg.AppConfigs["my-helm-app"].ConfigManagement.Helm.ValuesPaths,
	)
	require.Equal(
		t,
		val,
		cfg.AppConfigs["my-helm-app"].OutputPath,
	)
	require.Equal(
		t,
		[]string{val, val},
		cfg.AppConfigs["my-ytt-app"].ConfigManagement.Ytt.Paths,
	)
	require.Equal(
		t,
		val,
		cfg.AppConfigs["my-ytt-app"].OutputPath,
	)
	// Check that the original testCfg.AppConfigs haven't been touched.
	// References to maps are pointers, hence the extra care.
	require.Equal(
		t,
		"${0}",
		testCfg.AppConfigs["my-kustomize-app"].ConfigManagement.Kustomize.Path,
	)
	require.Equal(
		t,
		"${0}",
		testCfg.AppConfigs["my-helm-app"].ConfigManagement.Helm.ChartPath,
	)
	require.Equal(
		t,
		[]string{"${0}", "${0}"},
		testCfg.AppConfigs["my-helm-app"].ConfigManagement.Helm.ValuesPaths,
	)
	require.Equal(
		t,
		[]string{"${0}", "${0}"},
		testCfg.AppConfigs["my-ytt-app"].ConfigManagement.Ytt.Paths,
	)
}
