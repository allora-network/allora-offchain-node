package lib

import (
	"context"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) GetTopicById(topicId emissionstypes.TopicId) (emissionstypes.Topic, error) {
	ctx := context.Background()
	res, err := node.Chain.EmissionsQueryClient.GetTopic(ctx, &emissionstypes.QueryTopicRequest{
		TopicId: topicId,
	})
	if err != nil {
		return emissionstypes.Topic{}, err
	}
	return *res.Topic, nil
}
