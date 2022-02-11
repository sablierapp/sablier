package strategy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingleDynamicStrategy_ServeHTTP(t *testing.T) {

	for _, test := range SingleServiceTestCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, test.onDemandServiceResponses[0].body)
			}))

			defer mockServer.Close()

			dynamicStrategy := &DynamicStrategy{
				Name:     "whoami",
				Requests: []string{mockServer.URL},
				Next:     next,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			dynamicStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected.dynamic, recorder.Code)
		})
	}
}

func TestMultipleDynamicStrategy_ServeHTTP(t *testing.T) {
	for _, test := range MultipleServicesTestCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			urls := make([]string, len(test.onDemandServiceResponses))
			for responseIndex, response := range test.onDemandServiceResponses {
				response := response
				mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, response.body)
				}))
				defer mockServer.Close()

				urls[responseIndex] = mockServer.URL
			}
			dynamicStrategy := &DynamicStrategy{
				Name:     "whoami",
				Requests: urls,
				Next:     next,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			dynamicStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected.dynamic, recorder.Code)
		})
	}
}
