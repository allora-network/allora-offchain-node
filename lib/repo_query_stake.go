package lib

import (
	"context"

	cosmossdk_io_math "cosmossdk.io/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) GetReputerStakeInTopic(
	topicId emissionstypes.TopicId,
	reputer Address,
) (cosmossdk_io_math.Int, error) {
	ctx := context.Background()
	resp, err := node.Chain.EmissionsQueryClient.GetStakeFromReputerInTopicInSelf(ctx, &emissionstypes.GetStakeFromReputerInTopicInSelfRequest{
		ReputerAddress: reputer,
		TopicId:        topicId,
	})
	if err != nil {
		return cosmossdk_io_math.Int{}, err
	}
	return resp.Amount, nil
}
