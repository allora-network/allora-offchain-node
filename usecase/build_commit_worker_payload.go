package usecase

import (
	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func BuildCommitWorkerPayload(openNonce emissions.Nonce) (emissions.WorkerDataBundle, bool, error) {
	// TODO
	// 1. Compute inferences
	// 2. Compute forecasts
	// 3. Sign, organize into bundle, and commit bundle to chain using retries
	successfulCommit := true
	return emissions.WorkerDataBundle{}, successfulCommit, nil
}
