package client

import (
	"context"
	"errors"
	"os"
	"math/rand"
	"path/filepath"
	"log"
	"time"

	cosmossdk_io_math "cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	chainParams "github.com/allora-network/allora-chain/app/params"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"

	"github.com/ignite/cli/v28/ignite/pkg/cosmosaccount"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"

	"allora_offchain_node/types"
)

const NUM_WORKER_RETRIES = 5
const NUM_REPUTER_RETRIES = 5
const NUM_REGISTRATION_RETRIES = 3
const NUM_STAKING_RETRIES = 3
const NUM_WORKER_RETRY_MIN_DELAY = 0
const NUM_WORKER_RETRY_MAX_DELAY = 2
const NUM_REPUTER_RETRY_MIN_DELAY = 0
const NUM_REPUTER_RETRY_MAX_DELAY = 2
const NUM_REGISTRATION_RETRY_MIN_DELAY = 1
const NUM_REGISTRATION_RETRY_MAX_DELAY = 2
const NUM_STAKING_RETRY_MIN_DELAY = 1
const NUM_STAKING_RETRY_MAX_DELAY = 2
const REPUTER_TOPIC_SUFFIX = "/reputer"

type IndividualUserConfig struct {
	Wallet  types.WalletConfig
	Worker  *types.WorkerConfig
	Reputer *types.ReputerConfig
}

type NodeConfig struct {
	Worker  *types.WorkerConfig
	Reputer *types.ReputerConfig
	Wallet  types.WalletConfig
	Chain   types.ChainConfig
}

func getAlloraClient(config *IndividualUserConfig) (*cosmosclient.Client, error) {
	// create a allora client instance
	ctx := context.Background()
	userHomeDir, _ := os.UserHomeDir()
	alloraClientHome := filepath.Join(userHomeDir, ".allorad")
	if config.Wallet.AlloraHomeDir != "" {
		alloraClientHome = config.Wallet.AlloraHomeDir
	}

	// Check that the given home folder exist
	if _, err := os.Stat(alloraClientHome); errors.Is(err, os.ErrNotExist) {
		log.Println("Home directory does not exist, creating...")
		err = os.MkdirAll(alloraClientHome, 0755)
		if err != nil {
			log.Printf("Cannot create allora client home directory: %s. Error: %s", alloraClientHome, err)
			config.Wallet.SubmitTx = false
			return nil, err
		}
		log.Printf("Allora client home directory created: %s", alloraClientHome)
	}

	client, err := cosmosclient.New(ctx,
		cosmosclient.WithNodeAddress(config.Wallet.NodeRpc),
		cosmosclient.WithAddressPrefix(config.Wallet.AddressPrefix),
		cosmosclient.WithHome(alloraClientHome),
		cosmosclient.WithGas(config.Wallet.Gas),
		cosmosclient.WithGasAdjustment(config.Wallet.GasAdjustment),
	)
	if err != nil {
		log.Printf("Unable to create an allora blockchain client: %s", err)
		config.Wallet.SubmitTx = false
		return nil, err
	}
	return &client, nil
}

func (config *IndividualUserConfig) NewAlloraChain() (*NodeConfig, error) {
	config.Wallet.SubmitTx = true
	client, err := getAlloraClient(config)
	if err != nil {
		config.Wallet.SubmitTx = false
		return nil, err
	}
	var account cosmosaccount.Account
	// if we're giving a keyring ring name, with no mnemonic restore
	if config.Wallet.AddressRestoreMnemonic == "" && config.Wallet.AddressKeyName != "" {
		// get account from the keyring
		account, err = client.Account(config.Wallet.AddressKeyName)
		if err != nil {
			config.Wallet.SubmitTx = false
			log.Printf("could not retrieve account from keyring: %s", err)
		}
	} else if config.Wallet.AddressRestoreMnemonic != "" && config.Wallet.AddressKeyName != "" {
		// restore from mnemonic
		account, err = client.AccountRegistry.Import(config.Wallet.AddressKeyName, config.Wallet.AddressRestoreMnemonic, config.Wallet.AddressAccountPassphrase)
		if err != nil {
			if err.Error() == "account already exists" {
				account, err = client.Account(config.Wallet.AddressKeyName)
			}

			if err != nil {
				config.Wallet.SubmitTx = false
				log.Printf("could not restore account from mnemonic: %s", err)
			}
		}
	} else {
		log.Println("no allora account was loaded")
		return nil, nil
	}

	address, err := account.Address(config.Wallet.AddressPrefix)
	if err != nil {
		config.Wallet.SubmitTx = false
		log.Println("could not retrieve allora blockchain address, transactions will not be submitted to chain")
	} else {
		log.Printf("allora blockchain address loaded: %s", address)
	}

	// Create query client
	queryClient := emissionstypes.NewQueryClient(client.Context())

	// Create bank client
	bankClient := banktypes.NewQueryClient(client.Context())

	// this is terrible, no isConnected as part of this code path
	if client.Context().ChainID == "" {
		return nil, nil
	}

	alloraChain := types.ChainConfig{
		Address:              address,
		Account:              account,
		Client:               client,
		EmissionsQueryClient: queryClient,
		BankQueryClient:      bankClient,
	}

	Node := NodeConfig{
		Chain:   alloraChain,
		Wallet:  config.Wallet,
		Worker:  config.Worker,
		Reputer: config.Reputer,
	}

	registerWithBlockchain(&Node)

	return &Node, nil
}

func isNodeRegistered(node *NodeConfig, topicId uint64) (bool, error) {
	ctx := context.Background()

	var (
		res *emissionstypes.QueryIsWorkerRegisteredInTopicIdResponse
		err error
	)

	if node.Worker != nil {
		res, err = node.Chain.EmissionsQueryClient.IsWorkerRegisteredInTopicId(ctx, &emissionstypes.QueryIsWorkerRegisteredInTopicIdRequest{
			TopicId: topicId,
			Address: node.Wallet.Address,
		})
	} else if node.Reputer != nil {
		// register reputer
	} 

	if err != nil {
		return false, err
	}

	return res.IsRegistered, nil
}

func hasBalanceForRegistration(
	ctx context.Context,
	node *NodeConfig,
	registrationFee cosmossdk_io_math.Int,
) (bool, error) {
	resp, err := node.Chain.BankQueryClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: node.Chain.Address,
		Denom:   chainParams.DefaultBondDenom,
	})
	if err != nil {
		return false, err
	}
	return registrationFee.LTE(resp.Balance.Amount), nil
}

func registerWithBlockchain(node *NodeConfig) {
	ctx := context.Background()

	var (
		isReputer bool
		is_registered bool
	)
	if node.Worker != nil {
		isReputer = false
		log.Printf("Registering worker for topic %d", node.Worker.TopicId)
	} else if node.Reputer != nil {
		isReputer = true
		log.Printf("Registering reputer for topic %d", node.Reputer.TopicId)
	} else {
		log.Println("No worker or reputer to register")
	}

	moduleParams, err := node.Chain.EmissionsQueryClient.Params(ctx, &emissionstypes.QueryParamsRequest{})
	if err != nil {
		log.Printf("could not get chain params: %s", err)
	}

	is_registered, err = isNodeRegistered(node, node.Worker.TopicId)
	if err != nil {
		log.Printf("could not check if the node is already registered for topic, skipping: %s", err)
	}

	if !is_registered {
		hasBalance, err := hasBalanceForRegistration(ctx, node, moduleParams.Params.RegistrationFee)
		if err != nil {
			log.Printf("could not check if the node has enough balance to register, skipping: %s", err)
		}
		if !hasBalance {
			log.Println("node does not have enough balance to register, skipping.")
		}

		var topicId uint64
		if node.Worker.TopicId != 0 {
			topicId = node.Worker.TopicId
		} else {
			topicId = node.Reputer.TopicId
		}

		msg := &emissionstypes.MsgRegister{
			Sender:       node.Chain.Address,
			TopicId:      topicId,
			Owner:        node.Chain.Address,
			IsReputer:    isReputer,
		}
		res, err := node.SendDataWithRetry(ctx, msg, NUM_REGISTRATION_RETRIES,
			NUM_REGISTRATION_RETRY_MIN_DELAY, NUM_REGISTRATION_RETRY_MAX_DELAY, "register node")
		if err != nil {
			log.Printf("could not register the node with the Allora blockchain in topic %d: %s. Tx hash: %s", topicId, err, res.TxHash)
		} else {
			if isReputer {
				var initstake = node.Wallet.InitialStake
				if initstake > 0 {
					msg := &emissionstypes.MsgAddStake{
						Sender:  node.Wallet.Address,
						Amount:  cosmossdk_io_math.NewInt(initstake),
						TopicId: topicId,
					}
					res, err := node.SendDataWithRetry(ctx, msg, NUM_STAKING_RETRIES,
						NUM_STAKING_RETRY_MIN_DELAY, NUM_STAKING_RETRY_MAX_DELAY, "add stake")
					if err != nil {
						log.Printf("could not stake the node with the Allora blockchain in specified topic: %s. Tx hash: %s", err, res.TxHash)
					}
				} else {
					log.Println("No initial stake configured")
				}
			}
		}
	} else {
		log.Printf("node already registered for topic %d", node.Worker.TopicId)
	}
}

func (node *NodeConfig) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, MaxRetries, MinDelay, MaxDelay int, SuccessMsg string) (*cosmosclient.Response, error) {
	var txResp *cosmosclient.Response
	var err error
	for retryCount := 0; retryCount <= MaxRetries; retryCount++ {
		txResponse, err := node.Chain.Client.BroadcastTx(ctx, node.Chain.Account, req)
		txResp = &txResponse
		if err == nil {
			log.Printf("Success: %s, Tx Hash: %s", SuccessMsg, txResp.TxHash)
			break
		}
		// Log the error for each retry.
		log.Printf("Failed: %s, retrying... (Retry %d/%d)", SuccessMsg, retryCount, MaxRetries)
		// Generate a random number between MinDelay and MaxDelay
		randomDelay := rand.Intn(MaxDelay-MinDelay+1) + MinDelay
		// Apply exponential backoff to the random delay
		backoffDelay := randomDelay << retryCount
		// Wait for the calculated delay before retrying
		time.Sleep(time.Duration(backoffDelay) * time.Second)
	}
	return txResp, err
}
