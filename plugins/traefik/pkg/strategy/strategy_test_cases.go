package strategy

import "encoding/json"

type OnDemandServiceResponses struct {
	body   string
	status int
}
type ExpectedStatusForStrategy struct {
	dynamic  int
	blocking int
}

type TestCase struct {
	desc                     string
	onDemandServiceResponses []OnDemandServiceResponses
	expected                 ExpectedStatusForStrategy
}

var SingleServiceTestCases = []TestCase{
	{
		desc:                     "service is / keeps on starting",
		onDemandServiceResponses: GenerateServicesResponses(1, "starting"),
		expected: ExpectedStatusForStrategy{
			dynamic:  202,
			blocking: 503,
		},
	},
	{
		desc:                     "service is started",
		onDemandServiceResponses: GenerateServicesResponses(1, "started"),
		expected: ExpectedStatusForStrategy{
			dynamic:  200,
			blocking: 200,
		},
	},
	{
		desc:                     "ondemand service is in error",
		onDemandServiceResponses: GenerateServicesResponses(1, "error"),
		expected: ExpectedStatusForStrategy{
			dynamic:  500,
			blocking: 500,
		},
	},
}

func GenerateServicesResponses(count int, serviceBody string) []OnDemandServiceResponses {
	body, err := json.Marshal(SablierResponse{State: serviceBody, Error: "ko"})
	if err != nil {
		return nil
	}
	responses := make([]OnDemandServiceResponses, count)
	for i := 0; i < count; i++ {
		if serviceBody == "starting" || serviceBody == "started" {
			responses[i] = OnDemandServiceResponses{
				body:   string(body),
				status: 200,
			}
		} else {
			responses[i] = OnDemandServiceResponses{
				body:   string(body),
				status: 503,
			}
		}
	}
	return responses
}

var MultipleServicesTestCases = []TestCase{
	{
		desc:                     "all services are starting",
		onDemandServiceResponses: GenerateServicesResponses(5, "starting"),
		expected: ExpectedStatusForStrategy{
			dynamic:  202,
			blocking: 503,
		},
	},
	{
		desc:                     "one started others are starting",
		onDemandServiceResponses: append(GenerateServicesResponses(1, "starting"), GenerateServicesResponses(4, "started")...),
		expected: ExpectedStatusForStrategy{
			dynamic:  202,
			blocking: 503,
		},
	},
	{
		desc:                     "one starting others are started",
		onDemandServiceResponses: append(GenerateServicesResponses(4, "starting"), GenerateServicesResponses(1, "started")...),
		expected: ExpectedStatusForStrategy{
			dynamic:  202,
			blocking: 503,
		},
	},
	{
		desc: "one errored others are starting",
		onDemandServiceResponses: append(
			GenerateServicesResponses(2, "starting"),
			append(
				GenerateServicesResponses(1, "error"),
				GenerateServicesResponses(2, "starting")...,
			)...,
		),
		expected: ExpectedStatusForStrategy{
			dynamic:  500,
			blocking: 500,
		},
	},
	{
		desc: "one errored others are started",
		onDemandServiceResponses: append(
			GenerateServicesResponses(1, "error"),
			GenerateServicesResponses(4, "started")...,
		),
		expected: ExpectedStatusForStrategy{
			dynamic:  500,
			blocking: 500,
		},
	},
	{
		desc: "one errored others are mix of starting / started",
		onDemandServiceResponses: append(
			GenerateServicesResponses(2, "started"),
			append(
				GenerateServicesResponses(1, "error"),
				GenerateServicesResponses(2, "starting")...,
			)...,
		),
		expected: ExpectedStatusForStrategy{
			dynamic:  500,
			blocking: 500,
		},
	},
	{
		desc:                     "all are started",
		onDemandServiceResponses: GenerateServicesResponses(5, "started"),
		expected: ExpectedStatusForStrategy{
			dynamic:  200,
			blocking: 200,
		},
	},
}
