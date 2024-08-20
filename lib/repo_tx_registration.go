package lib

import (
	"context"

	"github.com/rs/zerolog/log"

	cosmossdk_io_math "cosmossdk.io/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

// True if the actor is ultimately, definitively registered for the specified topic, else False
// Idempotent in registration
func (node *NodeConfig) RegisterWorkerIdempotently(ctx context.Context, config WorkerConfig) bool {
	isRegistered, err := node.IsWorkerRegistered(ctx, config.TopicId)
	if err != nil {
		log.Error().Err(err).Msg("Could not check if the node is already registered for topic as worker, skipping")
	}
	if isRegistered {
		log.Info().Uint64("topicId", config.TopicId).Msg("Worker node already registered for topic")
		return true
	}

	moduleParams, err := node.Chain.EmissionsQueryClient.Params(ctx, &emissionstypes.QueryParamsRequest{})
	if err != nil {
		log.Error().Err(err).Msg("Could not get chain params for worker ")
	}

	balance, err := node.GetBalance(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Could not check if the worker node has enough balance to register, skipping")
		return false
	}
	if !balance.GTE(moduleParams.Params.RegistrationFee) {
		log.Error().Str("balance", balance.String()).Msg("Worker node does not have enough balance to register, skipping.")
		return false
	}

	msg := &emissionstypes.MsgRegister{
		Sender:    node.Chain.Address,
		TopicId:   config.TopicId,
		Owner:     node.Chain.Address,
		IsReputer: false,
	}
	res, err := node.SendDataWithRetry(ctx, msg, "Register worker node")
	if err != nil {
		log.Error().Err(err).Uint64("topic", config.TopicId).Str("txHash", res.TxHash).Msg("Could not register the worker node with the Allora blockchain")
		return false
	}

	return true
}

// True if the actor is ultimately, definitively registered for the specified topic with at least config.MinStake placed on topic, else False
// Actor may be either a worker or a reputer
// Idempotent in registration and stake addition
func (node *NodeConfig) RegisterAndStakeReputerIdempotently(ctx context.Context, config ReputerConfig) bool {
	isRegistered, err := node.IsReputerRegistered(ctx, config.TopicId)
	if err != nil {
		log.Error().Err(err).Msg("Could not check if the node is already registered for topic as reputer, skipping")
	}
	if isRegistered {
		log.Info().Uint64("topicId", config.TopicId).Msg("Reputer node already registered")
		return true
	}

	moduleParams, err := node.Chain.EmissionsQueryClient.Params(ctx, &emissionstypes.QueryParamsRequest{})
	if err != nil {
		log.Error().Err(err).Msg("Could not get chain params for reputer")
	}

	balance, err := node.GetBalance(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Could not check if the Reputer node has enough balance to register, skipping")
		return false
	}
	if !balance.GTE(moduleParams.Params.RegistrationFee) {
		log.Error().Msg("Reputer node does not have enough balance to register, skipping.")
		return false
	}

	msgRegister := &emissionstypes.MsgRegister{
		Sender:    node.Chain.Address,
		TopicId:   config.TopicId,
		Owner:     node.Chain.Address,
		IsReputer: true,
	}
	res, err := node.SendDataWithRetry(ctx, msgRegister, "Register reputer node")
	if err != nil {
		log.Error().Err(err).Uint64("topic", config.TopicId).Str("txHash", res.TxHash).Msg("Could not register the reputer node with the Allora blockchain")
		return false
	}

	stake, err := node.GetReputerStakeInTopic(ctx, config.TopicId, node.Chain.Address)
	if err != nil {
		log.Error().Err(err).Msg("Could not check if the reputer node has enough balance to stake, skipping")
		return false
	}
	minStake := cosmossdk_io_math.NewInt(config.MinStake)
	if minStake.LTE(stake) {
		log.Error().Msg("Reputer stake below minimum stake, skipping.")
		return true
	}

	msgAddStake := &emissionstypes.MsgAddStake{
		Sender:  node.Wallet.Address,
		Amount:  minStake,
		TopicId: config.TopicId,
	}
	res, err = node.SendDataWithRetry(ctx, msgAddStake, "Add reputer stake")
	if err != nil {
		log.Error().Err(err).Uint64("topic", config.TopicId).Str("txHash", res.TxHash).Msg("Could not stake the reputer node with the Allora blockchain in specified topic")
		return false
	}
	return true
}
