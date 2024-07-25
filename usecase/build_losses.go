package usecase

import (
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func BuildLosses(
	configOptions types.ConfigOptions,
	openNonce emissions.Nonce,
	truth types.Truth,
	workerBundles []emissions.WorkerDataBundle,
) (emissions.ReputerValueBundle, error) {

	// TODO

	return emissions.ReputerValueBundle{}, nil
}
