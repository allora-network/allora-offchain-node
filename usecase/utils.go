package usecase

import (
	"time"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) Wait(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
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
		len(vb.OneOutInfererForecasterValues) == 0 &&
		len(vb.ExtraData) == 0
}
