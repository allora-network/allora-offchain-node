package usecase

import (
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) BuildCommitWorkerPayload(nonce emissionstypes.BlockHeight) (bool, error) {
	// TODO
	// 1. Compute inferences
	// 2. Compute forecasts
	// 3. Sign, organize into bundle, and commit bundle to chain using retries
	successfulCommit := true
	return successfulCommit, nil
}
