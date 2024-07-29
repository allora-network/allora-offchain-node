package usecase

import (
	"allora_offchain_node/lib"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) PollOpenReputerNonce(reputerConfig lib.ReputerConfig) (emissions.Nonce, error) {
	// TODO
	// 1. Query chain for open reputer nonce using Ignite client
	// 2. Repeat with retries until successful query
	// 3. Return query result if available, else nil
	return emissions.Nonce{}, nil
}

// Note:
// This is already implemented in `spawn_actor_processes.go` at time of writing.
// However, we may benefit from abstracting out some of that logic.
// After all, there is repetition between worker and reputing polling strategies.
