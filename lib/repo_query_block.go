package lib

import (
	"context"
	"errors"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/types/query"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

func (node *NodeConfig) GetReputerValuesAtBlock(ctx context.Context, topicId emissionstypes.TopicId, nonce BlockHeight) (*emissionstypes.NetworkInferenceBundle, error) {
	walletConfig, err := node.ConnectionManager.GetWalletConfig()
	if err != nil {
		return nil, errorsmod.Wrapf(err, "Error getting wallet config")
	}
	resp, err := QueryDataWithRetry(
		ctx,
		walletConfig.MaxRetries,
		walletConfig.RetryDelay,
		func(ctx context.Context, req query.PageRequest) (*emissionstypes.GetNetworkInferencesAtBlockResponse, error) {
			return node.Chain.EmissionsQueryClient.GetNetworkInferencesAtBlock(ctx, &emissionstypes.GetNetworkInferencesAtBlockRequest{
				TopicId:                  topicId,
				BlockHeightLastInference: nonce,
			})
		},
		query.PageRequest{}, //nolint:exhaustruct
		"get reputer values at block",
		node,
	)
	if err != nil {
		return nil, err
	}

	if resp.NetworkInferences == nil {
		return nil, errorsmod.Wrapf(errors.New("no network inferences found"), "getting reputer values: no network inferences found at block %d", nonce)
	}

	if resp.NetworkInferences.Nonce == 0 {
		return nil, errorsmod.Wrapf(errors.New("invalid reputer request nonce found"),
			"getting reputer values: invalid reputer request nonce %d found at block %d",
			resp.NetworkInferences.Nonce, nonce)
	}

	if len(resp.NetworkInferences.InfererValues) == 0 {
		return nil, errorsmod.Wrapf(errors.New("no inferer values found"),
			"getting reputer values: no inferer values found at block %d",
			nonce)
	}

	return resp.NetworkInferences, nil
}
