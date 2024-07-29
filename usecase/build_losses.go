package usecase

import (
	"allora_offchain_node/lib"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) BuildLosses(
	openNonce emissions.Nonce,
	truth lib.Truth,
	workerBundles []emissions.WorkerDataBundle,
) (emissions.ReputerValueBundle, error) {

	// TODO

	return emissions.ReputerValueBundle{}, nil
}
