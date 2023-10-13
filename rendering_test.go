package render

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/akuity/kargo-render/internal/helm"
	"github.com/akuity/kargo-render/internal/kustomize"
	"github.com/akuity/kargo-render/internal/ytt"
)

func TestPreRender(t *testing.T) {
	const testAppName = "test-app"
	fakeManifest := []byte("fake-manifest")
	testCases := []struct {
		name       string
		rc         requestContext
		service    *service
		assertions func(manifests map[string][]byte, err error)
	}{
		{
			name: "error pre-rendering with helm",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							testAppName: {
								ConfigManagement: configManagementConfig{
									Helm: &helm.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				helmRenderFn: func(
					context.Context,
					string,
					string,
					string,
					[]string,
				) ([]byte, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(_ map[string][]byte, err error) {
				require.Error(t, err)
				require.Equal(t, "something went wrong", err.Error())
			},
		},
		{
			name: "success pre-rendering with helm",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							testAppName: {
								ConfigManagement: configManagementConfig{
									Helm: &helm.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				helmRenderFn: func(
					context.Context,
					string,
					string,
					string,
					[]string,
				) ([]byte, error) {
					return fakeManifest, nil
				},
			},
			assertions: func(manifests map[string][]byte, err error) {
				require.NoError(t, err)
				require.Equal(t, fakeManifest, manifests[testAppName])
			},
		},
		{
			name: "error pre-rendering with ytt",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							"test-app": {
								ConfigManagement: configManagementConfig{
									Ytt: &ytt.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				yttRenderFn: func(context.Context, []string) ([]byte, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(_ map[string][]byte, err error) {
				require.Error(t, err)
				require.Equal(t, "something went wrong", err.Error())
			},
		},
		{
			name: "success pre-rendering with ytt",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							"test-app": {
								ConfigManagement: configManagementConfig{
									Ytt: &ytt.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				yttRenderFn: func(context.Context, []string) ([]byte, error) {
					return fakeManifest, nil
				},
			},
			assertions: func(manifests map[string][]byte, err error) {
				require.NoError(t, err)
				require.Equal(t, fakeManifest, manifests[testAppName])
			},
		},
		{
			name: "error pre-rendering with kustomize",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							"test-app": {
								ConfigManagement: configManagementConfig{
									Kustomize: &kustomize.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				kustomizeRenderFn: func(
					context.Context,
					string,
					[]string,
					kustomize.Config,
				) ([]byte, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(manifests map[string][]byte, err error) {
				require.Error(t, err)
				require.Equal(t, "something went wrong", err.Error())
			},
		},
		{
			name: "success pre-rendering with kustomize",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							"test-app": {
								ConfigManagement: configManagementConfig{
									Kustomize: &kustomize.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				kustomizeRenderFn: func(
					context.Context,
					string,
					[]string,
					kustomize.Config,
				) ([]byte, error) {
					return fakeManifest, nil
				},
			},
			assertions: func(manifests map[string][]byte, err error) {
				require.NoError(t, err)
				require.Equal(t, fakeManifest, manifests[testAppName])
			},
		},
		{
			name: "safeguards against empty manifests",
			rc: requestContext{
				target: targetContext{
					branchConfig: branchConfig{
						AppConfigs: map[string]appConfig{
							"test-app": {
								ConfigManagement: configManagementConfig{
									Kustomize: &kustomize.Config{},
								},
							},
						},
					},
				},
			},
			service: &service{
				kustomizeRenderFn: func(
					context.Context,
					string,
					[]string,
					kustomize.Config,
				) ([]byte, error) {
					return []byte{}, nil // This is probably a mistake!
				},
			},
			assertions: func(manifests map[string][]byte, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"contain 0 bytes; this looks like a mistake",
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.rc.logger = &logrus.Entry{
				Logger: logrus.New(),
			}
			testCase.assertions(
				testCase.service.preRender(
					context.Background(),
					testCase.rc,
					"fake/repo/path",
				),
			)
		})
	}
}
