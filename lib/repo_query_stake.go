package lib

import (
	"context"

	cosmossdk_io_math "cosmossdk.io/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) GetReputerStakeInTopic(
	ctx context.Context,
	topicId emissionstypes.TopicId,
	reputer Address,
) (cosmossdk_io_math.Int, error) {
	resp, err := node.Chain.EmissionsQueryClient.GetStakeFromReputerInTopicInSelf(ctx, &emissionstypes.QueryStakeFromReputerInTopicInSelfRequest{
		ReputerAddress: reputer,
		TopicId:        topicId,
	})
	if err != nil {
		return cosmossdk_io_math.Int{}, err
	}
	return resp.Amount, nil
}
