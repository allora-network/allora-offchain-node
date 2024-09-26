package lib

import (
	"context"
	"errors"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) IsWorkerRegistered(ctx context.Context, topicId uint64) (bool, error) {
	var (
		res *emissionstypes.IsWorkerRegisteredInTopicIdResponse
		err error
	)

	if node.Worker != nil {
		res, err = node.Chain.EmissionsQueryClient.IsWorkerRegisteredInTopicId(ctx, &emissionstypes.IsWorkerRegisteredInTopicIdRequest{
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
		res *emissionstypes.IsReputerRegisteredInTopicIdResponse
		err error
	)

	if node.Reputer != nil {
		res, err = node.Chain.EmissionsQueryClient.IsReputerRegisteredInTopicId(ctx, &emissionstypes.IsReputerRegisteredInTopicIdRequest{
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
