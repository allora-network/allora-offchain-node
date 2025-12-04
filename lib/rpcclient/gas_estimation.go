package rpcclient

import (
	"fmt"
	"math"

	cosmossdk_io_math "cosmossdk.io/math"
)

// CalculateFees safely computes the fee amount.
func CalculateFees(gas uint64, minGasPrice float64) (cosmossdk_io_math.Int, error) {
	if gas == 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("gas cannot be zero")
	}
	if minGasPrice <= 0 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("minimum gas price must be greater than zero")
	}

	// Convert gas and gas price to fee with rounding
	floatFee := math.Round(float64(gas) * minGasPrice)
	if floatFee > math.MaxUint64 {
		return cosmossdk_io_math.NewInt(0), fmt.Errorf("fee overflows")
	}

	// Convert to uint safely
	uintFee := uint64(floatFee)
	fee := cosmossdk_io_math.NewIntFromUint64(uintFee)

	return fee, nil
}
