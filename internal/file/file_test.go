package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	file := "file_test.go"
	exists, err := Exists(file)
	require.NoError(t, err)
	require.True(t, exists)

	file = "bogus.go"
	exists, err = Exists(file)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestExpandPath(t *testing.T) {
	testCases := []struct {
		name           string
		pathTemplate   string
		values         []string
		expectedOutput string
	}{
		{
			name:           "empty string",
			pathTemplate:   "",
			values:         []string{"foo", "bar"},
			expectedOutput: "",
		},
		{
			name:           "single substitution",
			pathTemplate:   "this is a ${0} test",
			values:         []string{"foo", "bar"},
			expectedOutput: "this is a foo test",
		},
		{
			name:           "multiples substitutions",
			pathTemplate:   "this is a ${0} ${1} test",
			values:         []string{"foo", "bar"},
			expectedOutput: "this is a foo bar test",
		},
		{
			name:           "placeholder with no corresponding value",
			pathTemplate:   "this is a ${0} ${1} ${2} test",
			values:         []string{"foo", "bar"},
			expectedOutput: "this is a foo bar ${2} test",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(
				t,
				testCase.expectedOutput,
				ExpandPath(testCase.pathTemplate, testCase.values),
			)
		})
	}
}
