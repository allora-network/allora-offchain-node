package usecase

import (
	"context"
	"errors"
	"time"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

// DoneOrWait returns true if ctx.Done() arrived first
func (suite *UseCaseSuite) DoneOrWait(ctx context.Context, seconds int64) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(time.Duration(seconds) * time.Second):
		return false
	}
}

// Validations for Dec values
func ValidateDec(value alloraMath.Dec) error {
	if value.IsNaN() {
		return errors.New("value cannot be NaN")
	}

	if !value.IsFinite() {
		return errors.New("value must be finite")
	}

	return nil
}

func IsEmpty(vb emissionstypes.ValueBundle) bool {
	return vb.TopicId == 0 &&
		vb.ReputerRequestNonce == nil &&
		vb.Reputer == "" &&
		vb.CombinedValue.IsZero() &&
		vb.NaiveValue.IsZero() &&
		len(vb.InfererValues) == 0 &&
		len(vb.ForecasterValues) == 0 &&
		len(vb.OneOutInfererValues) == 0 &&
		len(vb.OneOutForecasterValues) == 0 &&
		len(vb.OneInForecasterValues) == 0 &&
		len(vb.ExtraData) == 0
}
