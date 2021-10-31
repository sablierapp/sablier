package traefik_ondemand_plugin

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
			desc: "invalid Config",
			config: &Config{
				ServiceUrl: "",
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
