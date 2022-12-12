package manifests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONStringsToYAMLBytes(t *testing.T) {
	testCases := []struct {
		name           string
		inputManifests []string
		assertions     func([][]byte, error)
	}{
		{
			name: "invalid JSON",
			inputManifests: []string{
				"{",
			},
			assertions: func(_ [][]byte, err error) {
				require.Error(t, err)
				require.Contains(
					t,
					err.Error(),
					"error converting JSON manifest to YAML",
				)
			},
		},
		{
			name: "valid JSON",
			inputManifests: []string{
				`{ "foo": "bar" }`,
				`{ "bat": "baz" }`,
			},
			assertions: func(manifests [][]byte, err error) {
				require.NoError(t, err)
				require.Len(t, manifests, 2)
				require.Equal(t, []byte("foo: bar\n"), manifests[0])
				require.Equal(t, []byte("bat: baz\n"), manifests[1])
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.assertions(JSONStringsToYAMLBytes(testCase.inputManifests))
		})
	}
}

func TestCombineYAML(t *testing.T) {
	combined := CombineYAML(
		[][]byte{
			[]byte("foo: bar\n"),
			[]byte("bat: baz\n"),
		},
	)
	require.Equal(
		t,
		[]byte("foo: bar\n---\nbat: baz\n"),
		combined,
	)
}

func TestSplitYAML(t *testing.T) {
	testCases := []struct {
		name       string
		manifests  []byte
		assertions func(map[string][]byte, error)
	}{
		{
			name:      "missing kind field",
			manifests: []byte("metadata:\n  name: foo\n"),
			assertions: func(_ map[string][]byte, err error) {
				require.Error(t, err)
				require.Equal(t, "resource is missing kind field", err.Error())
			},
		},
		{
			name:      "missing metadata.name field",
			manifests: []byte("kind: foo\n"),
			assertions: func(_ map[string][]byte, err error) {
				require.Error(t, err)
				require.Equal(t, "resource is missing metadata.name field", err.Error())
			},
		},
		{
			name: "success",
			manifests: []byte(`kind: foo
metadata:
  name: bar
---
kind: bat
metadata:
  name: baz
`),
			assertions: func(manifests map[string][]byte, err error) {
				require.NoError(t, err)
				require.Len(t, manifests, 2)
				require.Equal(
					t,
					[]byte(`kind: foo
metadata:
  name: bar
`),
					manifests["bar-foo"],
				)
				require.Equal(
					t,
					[]byte(`kind: bat
metadata:
  name: baz
`),
					manifests["baz-bat"],
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.assertions(SplitYAML(testCase.manifests))
		})
	}
}
