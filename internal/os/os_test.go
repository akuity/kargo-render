package os

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testEnvVarName = "FOO"

func TestGetEnvVar(t *testing.T) {
	const testDefaultVal = "default"
	testCases := []struct {
		name       string
		setup      func()
		assertions func(*testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				require.Equal(
					t,
					testDefaultVal,
					GetEnvVar(testEnvVarName, testDefaultVal),
				)
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "foo")
			},
			assertions: func(t *testing.T) {
				require.Equal(t, "foo", GetEnvVar(testEnvVarName, testDefaultVal))
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}

func TestGetRequiredEnvVar(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func()
		assertions func(t *testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				_, err := GetRequiredEnvVar(testEnvVarName)
				require.Error(t, err)
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "foo")
			},
			assertions: func(t *testing.T) {
				val, err := GetRequiredEnvVar(testEnvVarName)
				require.NoError(t, err)
				require.Equal(t, "foo", val)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}

func TestGetStringSliceFromEnvVar(t *testing.T) {
	testDefaultVal := []string{"foo", "bar"}
	testCases := []struct {
		name       string
		setup      func()
		assertions func(t *testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				val := GetStringSliceFromEnvVar(testEnvVarName, testDefaultVal)
				require.Equal(t, testDefaultVal, val)
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "a,b,c")
			},
			assertions: func(t *testing.T) {
				val := GetStringSliceFromEnvVar(testEnvVarName, testDefaultVal)
				require.Equal(t, []string{"a", "b", "c"}, val)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}

func TestGetIntFromEnvVar(t *testing.T) {
	const testDefaultVal = 1
	testCases := []struct {
		name       string
		setup      func()
		assertions func(t *testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				val, err := GetIntFromEnvVar(testEnvVarName, testDefaultVal)
				require.NoError(t, err)
				require.Equal(t, testDefaultVal, val)
			},
		},
		{
			name: "env var value not parsable as int",
			setup: func() {
				t.Setenv(testEnvVarName, "life, the universe, and everything")
			},
			assertions: func(t *testing.T) {
				_, err := GetIntFromEnvVar(testEnvVarName, testDefaultVal)
				require.Error(t, err)
				require.Contains(t, err.Error(), "was not parsable as an int")
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "42")
			},
			assertions: func(t *testing.T) {
				val, err := GetIntFromEnvVar(testEnvVarName, testDefaultVal)
				require.NoError(t, err)
				require.Equal(t, 42, val)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}

func TestGetBoolFromEnvVar(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func()
		assertions func(t *testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				val, err := GetBoolFromEnvVar(testEnvVarName, true)
				require.NoError(t, err)
				require.True(t, val)
			},
		},
		{
			name: "env var value not parsable as int",
			setup: func() {
				t.Setenv(testEnvVarName, "not really")
			},
			assertions: func(t *testing.T) {
				_, err := GetBoolFromEnvVar(testEnvVarName, false)
				require.Error(t, err)
				require.Contains(t, err.Error(), "was not parsable as a bool")
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "true")
			},
			assertions: func(t *testing.T) {
				val, err := GetBoolFromEnvVar(testEnvVarName, false)
				require.NoError(t, err)
				require.Equal(t, true, val)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}

func TestGetDurationFromEnvVar(t *testing.T) {
	const testDefaultVal = time.Minute
	testCases := []struct {
		name       string
		setup      func()
		assertions func(t *testing.T)
	}{
		{
			name: "env var does not exist",
			assertions: func(t *testing.T) {
				val, err := GetDurationFromEnvVar(testEnvVarName, testDefaultVal)
				require.NoError(t, err)
				require.Equal(t, testDefaultVal, val)
			},
		},
		{
			name: "env var value not parsable as duration",
			setup: func() {
				t.Setenv(testEnvVarName, "life, the universe, and everything")
			},
			assertions: func(t *testing.T) {
				_, err := GetDurationFromEnvVar(testEnvVarName, testDefaultVal)
				require.Error(t, err)
				require.Contains(t, err.Error(), "was not parsable as a duration")
			},
		},
		{
			name: "env var exists",
			setup: func() {
				t.Setenv(testEnvVarName, "20s")
			},
			assertions: func(t *testing.T) {
				val, err := GetDurationFromEnvVar(testEnvVarName, testDefaultVal)
				require.NoError(t, err)
				require.Equal(t, 20*time.Second, val)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
				testCase.setup()
			}
			testCase.assertions(t)
		})
	}
}
