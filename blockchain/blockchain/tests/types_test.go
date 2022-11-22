package tests

import (
	"IS/blockchain/blockchain/api"
	"IS/utils"
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

func TestToHex(t *testing.T) {
	str := utils.ToHex([]byte{177, 179, 119, 58, 5, 192, 237, 1, 118, 120, 122, 79, 21, 116, 255, 0, 117, 247, 82, 30})
	if str != "b1b3773a05c0ed0176787a4f1574ff0075f7521e" {
		t.Fatal()
	}
}
