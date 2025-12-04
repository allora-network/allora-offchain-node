package transaction

import (
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type TransactionParams struct {
	ChainID             string
	Denom               string
	Prefix              string
	Sequence            uint64
	AccNum              uint64
	PrivKey             cryptotypes.PrivKey
	PubKey              cryptotypes.PubKey
	TimeoutHeight       uint64
	GasEstimationConfig GasEstimationConfig
	FeeGranterAddress   string
}

// GasEstimationConfig holds the parameters used for gas estimation.
type GasEstimationConfig struct {
	MinGasPrice   float64 // Minimum gas price in the smallest denomination
	OverrideFees  uint64  // Override the gas price with a fixed value
	GasAdjustment float64 // Adjustment factor for the gas used
}

func (g *GasEstimationConfig) Validate() error {
	if g.GasAdjustment < 0 {
		return fmt.Errorf("gas adjustment cannot be negative")
	}
	if g.MinGasPrice < 0 {
		return fmt.Errorf("min gas price cannot be negative")
	}
	return nil
}

func (txParams *TransactionParams) Validate() error {
	if err := txParams.GasEstimationConfig.Validate(); err != nil {
		return err
	}
	if txParams.PrivKey == nil {
		return fmt.Errorf("private key cannot be nil")
	}
	if txParams.PubKey == nil {
		return fmt.Errorf("public key cannot be nil")
	}
	if txParams.ChainID == "" {
		return fmt.Errorf("chain ID cannot be empty")
	}
	if txParams.Denom == "" {
		return fmt.Errorf("denom cannot be empty")
	}
	if txParams.AccNum == 0 {
		return fmt.Errorf("account number cannot be 0")
	}
	return nil
}
