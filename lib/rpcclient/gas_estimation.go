package rpcclient

import (
	types "allora_offchain_node/lib/types"
	"fmt"
	"math"

	cosmossdk_io_math "cosmossdk.io/math"
)

// EstimateGas calculates the estimated gas for a transaction based on its size.
func EstimateGas(txSize int, config types.GasEstimationConfig) (uint64, error) {
	if txSize < 0 {
		return 0, fmt.Errorf("transaction size cannot be negative")
	}

	// Calculate gas for transaction size
	sizeGas := uint64(txSize) * config.GasPerByte

	// Total gas is base gas + size gas
	totalGas := config.BaseGas + sizeGas

	// Ensure totalGas is within reasonable bounds to avoid overflow
	if totalGas > math.MaxUint64 {
		return 0, fmt.Errorf("calculated gas exceeds maximum allowable value")
	}

	return totalGas, nil
}

// CalculateFees calculates the fee amount based on the estimated gas and the minimum gas price.
func CalculateFees(gas uint64, minGasPrice float64) (cosmossdk_io_math.Int, error) {
	if gas == 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("gas cannot be zero")
	}
	if minGasPrice <= 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("minimum gas price must be greater than zero")
	}

	// Convert gas and gas price to fee
	fee := cosmossdk_io_math.NewIntFromUint64(gas * uint64(minGasPrice))

	return fee, nil
}
