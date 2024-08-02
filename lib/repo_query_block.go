package lib

import (
	"context"
	"encoding/json"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/rs/zerolog/log"
)

func (node *NodeConfig) GetReputerValuesAtBlock(topicId emissionstypes.TopicId, nonce BlockHeight) (*emissionstypes.ValueBundle, error) {
	ctx := context.Background()
	lastReputerCommitValue := int64(0)
	lastReputerCommit, err := node.Chain.EmissionsQueryClient.GetTopicLastReputerCommitInfo(ctx, &emissionstypes.QueryTopicLastCommitRequest{TopicId: topicId})
	if err != nil {
		log.Warn().Err(err).Int64("height", nonce).Msg("Error getting last reputer commit, using 0 as last reputer commit")
	}

	if lastReputerCommit != nil && lastReputerCommit.LastCommit != nil && lastReputerCommit.LastCommit.Nonce != nil {
		lastReputerCommitValue = lastReputerCommit.LastCommit.Nonce.BlockHeight
	}
	req := &emissionstypes.QueryNetworkInferencesAtBlockRequest{
		TopicId:                  topicId,
		BlockHeightLastInference: nonce,
		BlockHeightLastReward:    lastReputerCommitValue,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling QueryNetworkInferencesAtBlockRequest to print Msg as JSON")
	} else {
		log.Info().Str("req", string(reqJSON)).Msg("Getting QueryNetworkInferencesAtBlockRequest from chain")
	}

	res, err := node.Chain.EmissionsQueryClient.GetNetworkInferencesAtBlock(ctx, req)
	if err != nil {
		return &emissionstypes.ValueBundle{}, err
	}

	return res.NetworkInferences, nil
}
