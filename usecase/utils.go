package usecase

import (
	"errors"
	"time"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) Wait(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
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
