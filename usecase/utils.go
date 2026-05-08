package usecase

import emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"

func IsEmpty(vb emissionstypes.NetworkInferenceBundle) bool {
	return vb.TopicId == 0 &&
		len(vb.CombinedValue) == 0 &&
		len(vb.NaiveValue) == 0 &&
		len(vb.InfererValues) == 0 &&
		len(vb.ForecasterValues) == 0 &&
		len(vb.OneOutInfererValues) == 0 &&
		len(vb.OneOutForecasterValues) == 0 &&
		len(vb.OneInForecasterValues) == 0 &&
		len(vb.OneOutInfererForecasterValues) == 0
}
