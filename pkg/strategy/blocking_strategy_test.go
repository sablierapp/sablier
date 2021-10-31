package strategy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlockingStrategy_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc     string
		body     string
		status   int
		expected int
	}{
		{
			desc:     "service keeps on starting",
			body:     "starting",
			status:   200,
			expected: 503,
		},
		{
			desc:     "service is started",
			body:     "started",
			status:   200,
			expected: 200,
		},
		{
			desc:     "ondemand service is in error",
			body:     "error",
			status:   503,
			expected: 500,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})

			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))

			defer mockServer.Close()

			blockingStrategy := &BlockingStrategy{
				Name:       "whoami",
				Request:    mockServer.URL,
				Next:       next,
				BlockDelay: 1 * time.Second,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			blockingStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected, recorder.Code)
		})
	}
}
