package rpcclient

import (
	"math"
	"math/big"
	"testing"

	cosmossdk_io_math "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestCalculateFees(t *testing.T) {
	tests := []struct {
		name        string
		gas         uint64
		minGasPrice float64
		expectFee   cosmossdk_io_math.Int
		expectError bool
		epsilon     cosmossdk_io_math.Int
	}{
		{
			name:        "Normal case",
			gas:         100000,
			minGasPrice: 0.1,
			expectFee:   cosmossdk_io_math.NewIntFromUint64(10000),
			expectError: false,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(0),
		},
		{
			name:        "Zero gas",
			gas:         0,
			minGasPrice: 0.1,
			expectFee:   cosmossdk_io_math.NewInt(0),
			expectError: true,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(0),
		},
		{
			name:        "Zero gas price",
			gas:         100000,
			minGasPrice: 0,
			expectFee:   cosmossdk_io_math.NewInt(0),
			expectError: true,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(0),
		},
		{
			name:        "Negative gas price",
			gas:         100000,
			minGasPrice: -0.1,
			expectFee:   cosmossdk_io_math.NewInt(0),
			expectError: true,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(0),
		},
		{
			name:        "Very small gas price",
			gas:         100000,
			minGasPrice: 0.000000001,
			expectFee:   cosmossdk_io_math.NewInt(0),
			expectError: false,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(0),
		},
		{
			name:        "Very large gas",
			gas:         math.MaxUint64,
			minGasPrice: 0.1,
			expectFee:   cosmossdk_io_math.NewIntFromUint64(1844674407370955161), // MaxUint64 * 0.1
			expectError: false,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(1000),
		},
		{
			name:        "Precise decimal gas price",
			gas:         100000,
			minGasPrice: 0.123456789,
			expectFee:   cosmossdk_io_math.NewIntFromUint64(12345),
			expectError: false,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(1),
		},
		{
			name:        "Both large values",
			gas:         math.MaxUint64,
			minGasPrice: math.MaxFloat64,
			expectFee:   cosmossdk_io_math.NewIntFromBigInt(big.NewInt(math.MaxInt64)),
			expectError: true,
			epsilon:     cosmossdk_io_math.NewIntFromUint64(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee, err := CalculateFees(tt.gas, tt.minGasPrice)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, tt.expectFee.Sub(fee).LTE(tt.epsilon))
		})
	}
}
