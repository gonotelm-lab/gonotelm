package algo

import (
	"math"
	"slices"
	"strings"
	"testing"
)

func TestPCAReduceVector(t *testing.T) {
	input := make([]float64, 1024)
	for i := range input {
		input[i] = math.Sin(float64(i)/13.0) + math.Cos(float64(i)/29.0) + float64(i%11)*0.03
	}
	inputCopy := append([]float64(nil), input...)

	got, err := PCAReduceVector(input, 32)
	if err != nil {
		t.Fatalf("PCAReduceVector failed: %v", err)
	}

	if !slices.Equal(input, inputCopy) {
		t.Fatalf("input vector should not be modified")
	}

	if len(got) != 32 {
		t.Fatalf("unexpected reduced length, got=%d want=32", len(got))
	}

	for i, v := range got {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("invalid value at %d: %v", i, v)
		}
	}
}

func TestPCAReduceVectorInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		input   []float64
		target  int
		wantErr string
	}{
		{
			name:    "empty input",
			input:   nil,
			target:  32,
			wantErr: "input vector is empty",
		},
		{
			name:    "target not positive",
			input:   []float64{1, 2, 3, 4},
			target:  0,
			wantErr: "greater than 0",
		},
		{
			name:    "target too small for pca",
			input:   []float64{1, 2, 3, 4},
			target:  1,
			wantErr: "at least 2",
		},
		{
			name:    "target not less than input",
			input:   []float64{1, 2, 3, 4},
			target:  4,
			wantErr: "less than input length",
		},
		{
			name:    "input not divisible by target",
			input:   []float64{1, 2, 3, 4, 5},
			target:  2,
			wantErr: "divisible",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := PCAReduceVector(tc.input, tc.target)
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error message, got=%q want contains %q", err.Error(), tc.wantErr)
			}
		})
	}
}

