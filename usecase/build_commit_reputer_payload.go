package usecase

import (
	"allora_offchain_node/lib"
)

func (suite *UseCaseSuite) BuildCommitReputerPayload(nonce lib.BlockHeight) (bool, error) {
	// TODO
	// 1. Fetch worker payloads associated with this nonce
	// 2. Fetch latest regrets for each participating worker at this nonce
	// 3. Compute network inferences and/or fetch from chain for this nonce
	// 4. Get ground truth
	// 5. Compute and return loss bundle. Example loss function: |y - x|
	// 6. Sign, organize into bundle, and commit bundle to chain using retries

	successfulCommit := true
	return successfulCommit, nil
}
