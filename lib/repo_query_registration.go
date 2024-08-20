package lib

import (
	"context"
	"errors"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) IsWorkerRegistered(ctx context.Context, topicId uint64) (bool, error) {
	var (
		res *emissionstypes.QueryIsWorkerRegisteredInTopicIdResponse
		err error
	)

	if node.Worker != nil {
		res, err = node.Chain.EmissionsQueryClient.IsWorkerRegisteredInTopicId(ctx, &emissionstypes.QueryIsWorkerRegisteredInTopicIdRequest{
			TopicId: topicId,
			Address: node.Wallet.Address,
		})
	} else {
		return false, errors.New("no worker to register")
	}

	if err != nil {
		return false, err
	}

	return res.IsRegistered, nil
}

func (node *NodeConfig) IsReputerRegistered(ctx context.Context, topicId uint64) (bool, error) {
	var (
		res *emissionstypes.QueryIsReputerRegisteredInTopicIdResponse
		err error
	)

	if node.Reputer != nil {
		res, err = node.Chain.EmissionsQueryClient.IsReputerRegisteredInTopicId(ctx, &emissionstypes.QueryIsReputerRegisteredInTopicIdRequest{
			TopicId: topicId,
			Address: node.Wallet.Address,
		})
	} else {
		return false, errors.New("no reputer to register")
	}

	if err != nil {
		return false, err
	}

	return res.IsRegistered, nil
}
