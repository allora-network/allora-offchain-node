package lib

import (
	"context"
	"log"

	cosmossdk_io_math "cosmossdk.io/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

// True if the actor is ultimately, definitively registered for the specified topic, else False
// Idempotent in registration
func (node *NodeConfig) RegisterWorkerIdempotently(config WorkerConfig) bool {
	ctx := context.Background()

	isRegistered, err := node.IsWorkerRegistered(config.TopicId)
	if err != nil {
		log.Printf("could not check if the node is already registered for topic as worker, skipping: %s", err)
	}
	if isRegistered {
		log.Printf("node already registered for topic %d", config.TopicId)
		return true
	}

	moduleParams, err := node.Chain.EmissionsQueryClient.Params(ctx, &emissionstypes.QueryParamsRequest{})
	if err != nil {
		log.Printf("could not get chain params: %s", err)
	}

	balance, err := node.GetBalance()
	if err != nil {
		log.Printf("could not check if the node has enough balance to register, skipping: %s", err)
		return false
	}
	if !balance.GTE(moduleParams.Params.RegistrationFee) {
		log.Println("node does not have enough balance to register, skipping.")
		return false
	}

	msg := &emissionstypes.MsgRegister{
		Sender:    node.Chain.Address,
		TopicId:   config.TopicId,
		Owner:     node.Chain.Address,
		IsReputer: false,
	}
	res, err := node.SendDataWithRetry(ctx, msg, "register node")
	if err != nil {
		log.Printf("could not register the node with the Allora blockchain in topic %d: %s. Tx hash: %s", config.TopicId, err, res.TxHash)
		return false
	}

	return true
}

// True if the actor is ultimately, definitively registered for the specified topic with at least config.MinStake placed on topic, else False
// Actor may be either a worker or a reputer
// Idempotent in registration and stake addition
func (node *NodeConfig) RegisterAndStakeReputerIdempotently(config ReputerConfig) bool {
	ctx := context.Background()

	isRegistered, err := node.IsReputerRegistered(config.TopicId)
	if err != nil {
		log.Printf("could not check if the node is already registered for topic as reputer, skipping: %s", err)
	}
	if isRegistered {
		log.Printf("node already registered for topic %d", config.TopicId)
		return true
	}

	moduleParams, err := node.Chain.EmissionsQueryClient.Params(ctx, &emissionstypes.QueryParamsRequest{})
	if err != nil {
		log.Printf("could not get chain params: %s", err)
	}

	balance, err := node.GetBalance()
	if err != nil {
		log.Printf("could not check if the node has enough balance to register, skipping: %s", err)
		return false
	}
	if !balance.GTE(moduleParams.Params.RegistrationFee) {
		log.Println("node does not have enough balance to register, skipping.")
		return false
	}

	msgRegister := &emissionstypes.MsgRegister{
		Sender:    node.Chain.Address,
		TopicId:   config.TopicId,
		Owner:     node.Chain.Address,
		IsReputer: true,
	}
	res, err := node.SendDataWithRetry(ctx, msgRegister, "register node")
	if err != nil {
		log.Printf("could not register the node with the Allora blockchain in topic %d: %s. Tx hash: %s", config.TopicId, err, res.TxHash)
		return false
	}

	stake, err := node.GetReputerStakeInTopic(config.TopicId, node.Chain.Address)
	if err != nil {
		log.Printf("could not check if the node has enough balance to stake, skipping: %s", err)
		return false
	}
	minStake := cosmossdk_io_math.NewInt(config.MinStake)
	if minStake.LTE(stake) {
		return true
	}

	msgAddStake := &emissionstypes.MsgAddStake{
		Sender:  node.Wallet.Address,
		Amount:  minStake,
		TopicId: config.TopicId,
	}
	res, err = node.SendDataWithRetry(ctx, msgAddStake, "add stake")
	if err != nil {
		log.Printf("could not stake the node with the Allora blockchain in specified topic: %s. Tx hash: %s", err, res.TxHash)
		return false
	}
	return true
}
