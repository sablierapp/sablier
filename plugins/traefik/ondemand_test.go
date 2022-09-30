package traefik

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOndemand(t *testing.T) {
	testCases := []struct {
		desc          string
		config        *Config
		expectedError bool
	}{
		{
			desc: "Invalid Config (no name)",
			config: &Config{
				ServiceUrl: "http://ondemand:1000",
				Timeout:    "1m",
			},
			expectedError: true,
		},
		{
			desc: "Invalid Config (empty names)",
			config: &Config{
				Names:      []string{},
				ServiceUrl: "http://ondemand:1000",
				Timeout:    "1m",
			},
			expectedError: true,
		},
		{
			desc: "Invalid Config (empty serviceUrl)",
			config: &Config{
				Name:       "whoami",
				ServiceUrl: "",
				Timeout:    "1m",
			},
			expectedError: true,
		},
		{
			desc: "Invalid Config (name and names used simultaneously)",
			config: &Config{
				Names: []string{
					"whoami-1", "whoami-2",
				},
				Name:       "whoami",
				ServiceUrl: "http://ondemand:1000",
				WaitUi:     true,
				BlockDelay: "1m",
				Timeout:    "1m",
			},
			expectedError: true,
		},
		{
			desc: "valid Dynamic Config",
			config: &Config{
				Name:       "whoami",
				ServiceUrl: "http://ondemand:1000",
				WaitUi:     true,
				Timeout:    "1m",
			},
			expectedError: false,
		},
		{
			desc: "valid Blocking Config",
			config: &Config{
				Name:       "whoami",
				ServiceUrl: "http://ondemand:1000",
				WaitUi:     false,
				BlockDelay: "1m",
				Timeout:    "1m",
			},
			expectedError: false,
		},
		{
			desc: "valid Dynamic Multiple Config",
			config: &Config{
				Names: []string{
					"whoami-1", "whoami-2",
				},
				ServiceUrl: "http://ondemand:1000",
				WaitUi:     false,
				BlockDelay: "1m",
				Timeout:    "1m",
			},
			expectedError: false,
		},
		{
			desc: "valid Blocking Multiple Config",
			config: &Config{
				Names: []string{
					"whoami-1", "whoami-2",
				},
				ServiceUrl: "http://ondemand:1000",
				WaitUi:     true,
				BlockDelay: "1m",
				Timeout:    "1m",
			},
			expectedError: false,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			ondemand, err := New(context.Background(), next, test.config, "traefikTest")

			if test.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, ondemand)
			}
		})
	}
}
