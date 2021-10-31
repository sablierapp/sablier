package strategy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDynamicStrategy_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc     string
		status   string
		expected int
	}{
		{
			desc:     "service is starting",
			status:   "starting",
			expected: 202,
		},
		{
			desc:     "service is started",
			status:   "started",
			expected: 200,
		},
		{
			desc:     "ondemand service is in error",
			status:   "error",
			expected: 500,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.status)
			}))

			defer mockServer.Close()

			dynamicStrategy := &DynamicStrategy{
				Name:    "whoami",
				Request: mockServer.URL,
				Next:    next,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			dynamicStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected, recorder.Code)
		})
	}
}
