package usecase

import (
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func PollOpenReputerNonce(config types.ConfigOptions, reputerConfig types.ReputerConfig) (emissions.Nonce, error) {
	// TODO
	// 1. Query chain for open reputer nonce using Ignite client
	// 2. Repeat with retries until successful query
	// 3. Return query result if available, else nil
	return emissions.Nonce{}, nil
}
