package usecase

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomJitter(t *testing.T) {
	tests := []struct {
		name             string
		submissionJitter uint64
		expectedMin      int64
		expectedMax      int64
		iterations       int
	}{
		{
			name:             "Zero jitter",
			submissionJitter: 0,
			expectedMin:      0,
			expectedMax:      0,
			iterations:       1000,
		},
		{
			name:             "Small jitter",
			submissionJitter: 10,
			expectedMin:      0,
			expectedMax:      9, // Since it's modulo, max will be submissionJitter - 1
			iterations:       10000,
		},
		{
			name:             "Medium jitter",
			submissionJitter: 100,
			expectedMin:      0,
			expectedMax:      99,
			iterations:       10000,
		},
		{
			name:             "Large jitter",
			submissionJitter: 1000,
			expectedMin:      0,
			expectedMax:      999,
			iterations:       10000,
		},
		{
			name:             "Power of two jitter",
			submissionJitter: 256,
			expectedMin:      0,
			expectedMax:      255,
			iterations:       10000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for range make([]struct{}, test.iterations) {
				result := generateRandomJitter(test.submissionJitter)
				fmt.Println(result)

				// Check bounds
				assert.GreaterOrEqual(t, result, test.expectedMin,
					"Result should be greater than or equal to the minimum value")
				assert.LessOrEqual(t, result, test.expectedMax,
					"Result should be less than or equal to the maximum value")
			}
		})
	}
}
