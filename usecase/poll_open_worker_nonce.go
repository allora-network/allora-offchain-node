package usecase

import (
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func PollOpenWorkerNonce(configOptions types.ConfigOptions, workerConfig types.WorkerConfig) (emissions.Nonce, error) {
	// TODO
	// 1. Query chain for open worker nonce using Ignite client
	// 2. Repeat with retries until successful query
	// 3. Return query result if available, else nil

	return emissions.Nonce{}, nil
}
