package tests

import (
	"IS/blockchain/blockchain/types"
	"testing"
)

func TestValueMinus(t *testing.T) {
	v1 := types.Value_{Integer: 1}
	v2 := types.Value_{Fractional: 1}

	v3 := v1.Minus(&v2)

	if v3.Integer != 0 || v3.Fractional != 99 {
		t.Fatal("Invalid result", v3)
	}
}
