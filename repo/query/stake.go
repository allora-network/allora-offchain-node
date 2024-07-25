package query

import (
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func GetReputerStakeInTopic(topicId emissions.TopicId, reputer types.Address) (types.Allo, error) {
	// TODO
	// Get stake of reputer in topic excluding delegate stake
	return 0, nil
}
