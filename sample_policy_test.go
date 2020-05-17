package failure_detector

import (
	"fmt"
	"testing"
)

func TestSimpleWeightedEndpointStatusEvaluator(t *testing.T) {
	scenarios := []struct {
		name               string
		endpoint           *WeightedEndpointStatus
		expectedStatus     string
		expectedWeight     float32
		expectedHasChanged bool
		steps              []struct {
			data               []*Sample
			expectedStatus     string
			expectedWeight     float32
			expectedHasChanged bool
		}
	}{
		{
			name:               "an endpoint received 10 errors - status NOT set, weight set to 0.90",
			endpoint:           createWeightedEndpointStatus(errToSampleFunc(genErrors(10)...)),
			expectedWeight:     0.90,
			expectedHasChanged: true,
		},
		{
			name:           "an endpoint received 4 errors - status NOT set, weight set to 1",
			endpoint:       createWeightedEndpointStatus(errToSampleFunc(genErrors(4)...)),
			expectedWeight: 1,
		},
		{
			name:           "an endpoint received 0 errors - status NOT set, weight set to 1",
			endpoint:       createWeightedEndpointStatus(errToSampleFunc(genNilErrors(1)...)),
			expectedWeight: 1,
		},
		{
			name: "an endpoint received a non-error sample after receiving 10 errors - status NOT set, weight set to 1.0",
			endpoint: func() *WeightedEndpointStatus {
				errors := genErrors(10)
				errors = append(errors, genNilErrors(1)...)
				ep := createWeightedEndpointStatus(errToSampleFunc(errors...))
				return ep
			}(),
			expectedWeight: 1.0,
		},

		{
			name:           "no errors",
			endpoint:       createWeightedEndpointStatus(errToSampleFunc(genNilErrors(10)...)),
			expectedWeight: 1.0,
			steps: []struct {
				data               []*Sample
				expectedStatus     string
				expectedWeight     float32
				expectedHasChanged bool
			}{
				// step 1
				{data: errToSampleFunc(genNilErrors(10)...), expectedWeight: 1.0},

				// step 2
				{data: errToSampleFunc(genNilErrors(10)...), expectedWeight: 1.0},
			},
		},

		{
			name:               "up and down",
			endpoint:           createWeightedEndpointStatus(errToSampleFunc(genErrors(10)...)),
			expectedWeight:     0.90,
			expectedHasChanged: true,
			steps: []struct {
				data               []*Sample
				expectedStatus     string
				expectedWeight     float32
				expectedHasChanged bool
			}{
				// step 1 - additional err doesn't decrease / increase the weight
				{data: errToSampleFunc(genErrors(1)...), expectedWeight: 0.90},

				// step 2 - additional 9 err do decrease the weight
				{data: errToSampleFunc(genErrors(9)...), expectedWeight: 0.80, expectedHasChanged: true},

				// step 3 - one success doesn't increase the weight
				{data: errToSampleFunc(genNilErrors(1)...), expectedWeight: 0.80},

				// step 4 - additional 8 don't as well
				{data: errToSampleFunc(genNilErrors(8)...), expectedWeight: 0.80},

				// step 5 - one more actually do (10 in total)
				{data: errToSampleFunc(genNilErrors(1)...), expectedWeight: 0.90, expectedHasChanged: true},

				// step 6 - one additional success doesn't change the weight
				{data: errToSampleFunc(genNilErrors(1)...), expectedWeight: 0.90},
			},
		},

		{
			name:               "drain",
			endpoint:           createWeightedEndpointStatus(errToSampleFunc(genErrors(10)...)),
			expectedWeight:     0.90,
			expectedHasChanged: true,
			steps: []struct {
				data               []*Sample
				expectedStatus     string
				expectedWeight     float32
				expectedHasChanged bool
			}{
				// step 1 - additional err doesn't decrease / increase the weight
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.80, expectedHasChanged: true},

				// step 2
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.70, expectedHasChanged: true},

				// step 3
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.60, expectedHasChanged: true},

				// step 4
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.50, expectedHasChanged: true},

				// step 5
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.40, expectedHasChanged: true},

				// step 6
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.30, expectedHasChanged: true},

				// step 7
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.20, expectedHasChanged: true},

				// step 8
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.10, expectedHasChanged: true},

				// step 9
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.0, expectedHasChanged: true, expectedStatus: EndpointStatusReasonTooManyErrors},

				// step 10
				{data: errToSampleFunc(genErrors(10)...), expectedWeight: 0.0, expectedStatus: EndpointStatusReasonTooManyErrors},

				// step 11
				{data: errToSampleFunc(genNilErrors(10)...), expectedWeight: 0.10, expectedHasChanged: true},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// test data
			hasChanged := SimpleWeightedEndpointStatusEvaluator(scenario.endpoint)

			validate := func(actualHasChanged bool, expectedHasChanged bool, expectedStatus string, expectedWeight float32) {
				if actualHasChanged != expectedHasChanged {
					t.Fatalf("expected the method to return %v value bot got %v value", expectedHasChanged, actualHasChanged)
				}
				if scenario.endpoint.status != expectedStatus {
					t.Fatalf("expected to get %s status but got %s", expectedStatus, scenario.endpoint.status)
				}
				if weightToErrorCount(scenario.endpoint.weight) != weightToErrorCount(expectedWeight) {
					t.Fatalf("expected to get %d errors based on %f weight but got %d errors based on %f", weightToErrorCount(expectedWeight), expectedWeight, weightToErrorCount(scenario.endpoint.weight), scenario.endpoint.weight)
				}
			}

			validate(hasChanged, scenario.expectedHasChanged, scenario.expectedStatus, scenario.expectedWeight)

			for _, step := range scenario.steps {
				for _, sample := range step.data {
					scenario.endpoint.Add(sample)
				}
				hasChanged := SimpleWeightedEndpointStatusEvaluator(scenario.endpoint)
				validate(hasChanged, step.expectedHasChanged, step.expectedStatus, step.expectedWeight)
			}
		})
	}
}

func createWeightedEndpointStatus(samples []*Sample) *WeightedEndpointStatus {
	target := newWeightedEndpoint(10, nil)
	for _, sample := range samples {
		target.Add(sample)
	}
	return target
}

func errToSampleFunc(err ...error) []*Sample {
	ret := []*Sample{}
	for _, e := range err {
		ret = append(ret, &Sample{err: e})
	}
	return ret
}

func genErrors(number int) []error {
	ret := make([]error, number)
	for i := 0; i < number; i++ {
		ret[i] = fmt.Errorf("error %d", i)
	}
	return ret
}

func genNilErrors(number int) []error {
	ret := make([]error, number)
	for i := 0; i < number; i++ {
		ret[i] = nil
	}
	return ret
}
