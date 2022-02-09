package strategy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSingleBlockingStrategy_ServeHTTP(t *testing.T) {
	for _, test := range SingleServiceTestCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})

			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.onDemandServiceResponses[0].status)
				fmt.Fprint(w, test.onDemandServiceResponses[0].body)
			}))

			defer mockServer.Close()

			blockingStrategy := &BlockingStrategy{
				Name:       "whoami",
				Requests:   []string{mockServer.URL},
				Next:       next,
				BlockDelay: 1 * time.Second,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			blockingStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected.blocking, recorder.Code)
		})
	}
}

func TestMultipleBlockingStrategy_ServeHTTP(t *testing.T) {

	for _, test := range MultipleServicesTestCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			urls := make([]string, len(test.onDemandServiceResponses))
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})

			for responseIndex, response := range test.onDemandServiceResponses {
				response := response
				mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(response.status)
					fmt.Fprint(w, response.body)
				}))

				defer mockServer.Close()
				urls[responseIndex] = mockServer.URL
			}
			fmt.Println(urls)
			blockingStrategy := &BlockingStrategy{
				Name:       "whoami",
				Requests:   urls,
				Next:       next,
				BlockDelay: 1 * time.Second,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			blockingStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected.blocking, recorder.Code)
		})
	}
}
