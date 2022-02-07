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
				Name:     "whoami",
				Requests: []string{mockServer.URL},
				Next:     next,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			dynamicStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected, recorder.Code)
		})
	}
}

func GenerateServicesStatuses(count int, status string) []string {
	statuses := make([]string, count)
	for i := 0; i < count; i++ {
		statuses[i] = status
	}
	return statuses
}

func TestMultipleDynamicStrategy_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc     string
		statuses []string
		expected int
	}{
		{
			desc:     "all services are starting",
			statuses: GenerateServicesStatuses(5, "starting"),
			expected: 202,
		},
		{
			desc:     "one started others are starting",
			statuses: append(GenerateServicesStatuses(1, "starting"), GenerateServicesStatuses(4, "started")...),
			expected: 202,
		},
		{
			desc:     "one starting others are started",
			statuses: append(GenerateServicesStatuses(4, "starting"), GenerateServicesStatuses(1, "started")...),
			expected: 202,
		},
		{
			desc: "one errored others are starting",
			statuses: append(
				GenerateServicesStatuses(2, "starting"),
				append(
					GenerateServicesStatuses(1, "error"),
					GenerateServicesStatuses(2, "starting")...,
				)...,
			),
			expected: 500,
		},
		{
			desc: "one errored others are started",
			statuses: append(
				GenerateServicesStatuses(1, "error"),
				GenerateServicesStatuses(4, "started")...,
			),
			expected: 500,
		},
		{
			desc: "one errored others are mix of starting / started",
			statuses: append(
				GenerateServicesStatuses(2, "started"),
				append(
					GenerateServicesStatuses(1, "error"),
					GenerateServicesStatuses(2, "starting")...,
				)...,
			),
			expected: 500,
		},
		{
			desc:     "all are started",
			statuses: GenerateServicesStatuses(5, "started"),
			expected: 200,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			urls := make([]string, len(test.statuses))
			for statusIndex, status := range test.statuses {
				status := status
				mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, status)
				}))
				defer mockServer.Close()

				urls[statusIndex] = mockServer.URL
			}
			dynamicStrategy := &DynamicStrategy{
				Name:     "whoami",
				Requests: urls,
				Next:     next,
			}

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "http://mydomain/whoami", nil)

			dynamicStrategy.ServeHTTP(recorder, req)

			assert.Equal(t, test.expected, recorder.Code)
		})
	}
}
