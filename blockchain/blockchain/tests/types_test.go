package tests

import (
	"IS/blockchain/blockchain/api"
	"testing"
)

type TestCase struct {
	v1             api.Value_
	v2             api.Value_
	expectedResult api.Value_
}

func TestValueMinus(t *testing.T) {
	cases := []TestCase{
		{
			v1:             api.Value_{Integer: 1},
			v2:             api.Value_{Fractional: 1},
			expectedResult: api.Value_{Fractional: 99},
		},

		{
			v1:             api.Value_{Integer: 10, Fractional: 50},
			v2:             api.Value_{Integer: 1, Fractional: 50},
			expectedResult: api.Value_{Integer: 9},
		},
	}

	for _, testCase := range cases {
		if testCase.v1.Minus(&testCase.v2) != testCase.expectedResult {
			t.Error("Invalid result")
		}
	}
}
