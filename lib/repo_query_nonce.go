package lib

import (
	"context"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) GetLatestOpenWorkerNonceByTopicId(topicId emissionstypes.TopicId) (BlockHeight, error) {
	// TODO: GetUnfulfilledWorkerNonces / GetOpenWorkerNonce
	// Must be deployed on a chain first!
	ctx := context.Background()

	res, err := node.Chain.EmissionsQueryClient.GetUnfulfilledWorkerNonces(
		ctx,
		&emissionstypes.QueryUnfulfilledWorkerNoncesRequest{TopicId: topicId},
	)
	if err != nil {
		return 0, err
	}

	if len(res.Nonces.Nonces) == 0 {
		return 0, nil
	}
	// Per `AddWorkerNonce()` in `allora-chain/x/emissions/keeper.go`, the latest nonce is first
	return res.Nonces.Nonces[0].BlockHeight, nil
}

func (node *NodeConfig) GetLatestOpenReputerNonceByTopicId(topicId emissionstypes.TopicId) (BlockHeight, error) {
	// TODO: GetUnfulfilledReputerNonces -> GetOpenReputerNonce
	// Must be deployed on a chain first!
	ctx := context.Background()

	res, err := node.Chain.EmissionsQueryClient.GetUnfulfilledReputerNonces(
		ctx,
		&emissionstypes.QueryUnfulfilledReputerNoncesRequest{TopicId: topicId},
	)
	if err != nil {
		return 0, err
	}

	if len(res.Nonces.Nonces) == 0 {
		return 0, nil
	}
	// Per `AddWorkerNonce()` in `allora-chain/x/emissions/keeper.go`, the latest nonce is first
	return res.Nonces.Nonces[0].ReputerNonce.BlockHeight, nil
}