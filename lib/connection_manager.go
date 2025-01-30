package lib

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"

	errorsmod "cosmossdk.io/errors"
	"github.com/rs/zerolog/log"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type RPCManagerInterface interface {
	GetCurrentQueryNode() *NodeConfig
	GetCurrentTxNode() *NodeConfig
	GetCurrentQueryIndex() int
	GetCurrentTxIndex() int
	SwitchToNextQueryNode() *NodeConfig
	SwitchToNextTxNode() *NodeConfig
	SwitchToQueryNode(index int) *NodeConfig
	SwitchToTxNode(index int) *NodeConfig
	SendDataWithNodeRetry(ctx context.Context, msg sdk.Msg, timeoutHeight uint64, operationName string) (*coretypes.ResultBroadcastTx, error)
	SendDataWithRetry(ctx context.Context, req sdk.Msg, infoMsg string, timeoutHeight uint64) (*coretypes.ResultBroadcastTx, error)
	GetQueryNodes() ([]NodeConfig, error)
	GetTxNodes() ([]NodeConfig, error)
	GetWallet() (*Wallet, error)
	GetWalletConfig() (*WalletConfig, error)
	Close() error
}

const (
	GRPC_MODE int = 1
	RPC_MODE  int = 2
)

type ConnectionManager struct {
	queryNodes   []NodeConfig
	txNodes      []NodeConfig
	queryIdx     int
	txIdx        int
	queryMu      sync.RWMutex
	txMu         sync.RWMutex
	wallet       *Wallet
	walletInit   sync.Once
	walletConfig *WalletConfig
}

func NewRPCManager(ctx context.Context, userConfig UserConfig) (*ConnectionManager, error) {
	if len(userConfig.Wallet.NodeRPCs) == 0 {
		return nil, fmt.Errorf("no GRPC/RPC nodes provided")
	}

	wallet, err := NewWalletFromConfig(ctx, userConfig.Wallet)
	if err != nil {
		return nil, err
	}

	// validate all urls are correct
	err = validateNodeURIs(userConfig.Wallet.NodeRPCs)
	if err != nil {
		return nil, err
	}

	// validate all urls are correct
	err = validateNodeURIs(userConfig.Wallet.NodeGRPCs)
	if err != nil {
		return nil, err
	}

	// Create a new RPCManager with the wallet and wallet config, will be
	var rpcManager = &ConnectionManager{
		wallet:       wallet,
		walletConfig: &userConfig.Wallet,
	}

	// Load here the nodeconfigs
	var queryNodes []NodeConfig
	for _, endpoint := range userConfig.Wallet.NodeGRPCs {
		log.Info().Str("grpc", endpoint).Msg("Initializing grpc query nodes")
		nodeConfig, err := userConfig.GenerateNodeConfig(ctx, wallet, GRPC_MODE, endpoint)
		if err != nil {
			log.Error().Err(err).Str("grpc", endpoint).Msg("Error generating node config, skipping GRPC node")
			continue
		}
		nodeConfig.RPCManager = rpcManager
		queryNodes = append(queryNodes, *nodeConfig)
	}

	var txNodes []NodeConfig

	for _, endpoint := range userConfig.Wallet.NodeRPCs {
		log.Info().Str("rpc", endpoint).Msg("Initializing rpc tx nodes")
		nodeConfig, err := userConfig.GenerateNodeConfig(ctx, wallet, RPC_MODE, endpoint)
		if err != nil {
			log.Error().Err(err).Str("rpc", endpoint).Msg("Error generating node config, skipping RPC node")
			continue
		}
		nodeConfig.RPCManager = rpcManager
		txNodes = append(txNodes, *nodeConfig)
	}

	rpcManager.queryNodes = queryNodes
	rpcManager.txNodes = txNodes
	rpcManager.queryIdx = 0
	rpcManager.txIdx = 0

	// Initialize the wallet with the account info
	_, sequence, accNum, err := rpcManager.GetCurrentQueryNode().GetAccountInfo(ctx, wallet.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}
	wallet.SetSequence(sequence)
	wallet.AccountNumber = accNum
	log.Info().Msgf("Wallet initialized successfully, with account (sequence: %d, accNum: %d)", sequence, accNum)

	return rpcManager, nil
}

func (r *ConnectionManager) InitializeWallet(ctx context.Context, walletConfig WalletConfig) error {
	var initErr error
	r.walletInit.Do(func() {
		wallet, err := NewWalletFromConfig(ctx, walletConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize wallet: %w", err)
			return
		}
		r.wallet = wallet
	})
	return initErr
}

// GetWallet returns the wallet instance, returns error if wallet is not initialized
func (r *ConnectionManager) GetWallet() (*Wallet, error) {
	if r.wallet == nil {
		return nil, fmt.Errorf("wallet not initialized")
	}
	return r.wallet, nil
}

func (r *ConnectionManager) GetWalletConfig() (*WalletConfig, error) {
	if r.walletConfig == nil {
		return nil, fmt.Errorf("wallet config not initialized")
	}
	return r.walletConfig, nil
}

func (r *ConnectionManager) GetQueryNodes() ([]NodeConfig, error) {
	r.queryMu.RLock()
	defer r.queryMu.RUnlock()
	return r.queryNodes, nil
}

func (r *ConnectionManager) GetTxNodes() ([]NodeConfig, error) {
	r.txMu.RLock()
	defer r.txMu.RUnlock()
	return r.txNodes, nil
}

func (r *ConnectionManager) GetCurrentQueryIndex() int {
	r.queryMu.RLock()
	defer r.queryMu.RUnlock()
	return r.queryIdx
}

func (r *ConnectionManager) GetCurrentTxIndex() int {
	r.txMu.RLock()
	defer r.txMu.RUnlock()
	return r.txIdx
}

func (r *ConnectionManager) GetCurrentQueryNode() *NodeConfig {
	r.queryMu.RLock()
	defer r.queryMu.RUnlock()
	return &r.queryNodes[r.queryIdx]
}

func (r *ConnectionManager) GetCurrentTxNode() *NodeConfig {
	r.txMu.RLock()
	defer r.txMu.RUnlock()
	return &r.txNodes[r.txIdx]
}

// internal function, switches to a node assuming a lock has been acquired
func (r *ConnectionManager) switchToNodeLocked(mode, index int, nodes []NodeConfig) *NodeConfig {
	if len(nodes) == 1 {
		return &nodes[0]
	}
	var oldIndex int
	if mode == GRPC_MODE {
		oldIndex = r.queryIdx
		r.queryIdx = index
	} else if mode == RPC_MODE {
		oldIndex = r.txIdx
		r.txIdx = index
	} else {
		log.Error().Int("mode", mode).Msg("Invalid mode, not switching")
		return nil
	}

	log.Debug().
		Str("from", nodes[oldIndex].ServerAddress).
		Str("to", nodes[index].ServerAddress).
		Msg("Switch to next RPC node")

	return &nodes[index]
}

// SwitchToNextNode switches to the next node in the list.
// Node change is persistent, so it will be used again in the next call
func (r *ConnectionManager) SwitchToNextQueryNode() *NodeConfig {
	r.queryMu.Lock()
	defer r.queryMu.Unlock()
	// Get next node index, wrap around if necessary
	nextNode := (r.queryIdx + 1) % len(r.queryNodes)
	return r.switchToNodeLocked(GRPC_MODE, nextNode, r.queryNodes)
}

func (r *ConnectionManager) SwitchToNextTxNode() *NodeConfig {
	r.txMu.Lock()
	defer r.txMu.Unlock()
	// Get next node index, wrap around if necessary
	nextNode := (r.txIdx + 1) % len(r.txNodes)
	return r.switchToNodeLocked(RPC_MODE, nextNode, r.txNodes)
}

// Switches to a specific node, acquiring a lock
func (r *ConnectionManager) SwitchToQueryNode(index int) *NodeConfig {
	r.queryMu.Lock()
	defer r.queryMu.Unlock()
	return r.switchToNodeLocked(GRPC_MODE, index, r.queryNodes)
}

func (r *ConnectionManager) SwitchToTxNode(index int) *NodeConfig {
	r.txMu.Lock()
	defer r.txMu.Unlock()
	return r.switchToNodeLocked(RPC_MODE, index, r.txNodes)
}

func (r *ConnectionManager) Close() error {
	log.Info().Msg("Closing RPCManager")
	// Iterate through all nodes and close them
	for _, node := range r.queryNodes {
		if node.Chain.GRPCClient != nil {
			err := node.Chain.GRPCClient.Close()
			if err != nil {
				return err
			}
		}
		if node.Chain.RPCClient != nil && node.Chain.RPCClient.Client != nil {
			err := node.Chain.RPCClient.Client.Stop()
			if err != nil {
				return err
			}
		}
	}

	for _, node := range r.txNodes {
		if node.Chain.GRPCClient != nil {
			err := node.Chain.GRPCClient.Close()
			if err != nil {
				return err
			}
		}
		if node.Chain.RPCClient != nil && node.Chain.RPCClient.Client != nil {
			err := node.Chain.RPCClient.Client.Stop()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ConnectionManager) SendDataWithNodeRetry(
	ctx context.Context,
	msg sdk.Msg,
	timeoutHeight uint64,
	operationName string,
) (*coretypes.ResultBroadcastTx, error) {
	return RunWithNodeRetry(ctx, r, func(node *NodeConfig) (*coretypes.ResultBroadcastTx, error) {
		return r.SendDataWithRetry(ctx, msg, operationName, timeoutHeight)
	}, operationName, RPC_MODE)
}

// RunWithNodeRetry executes an operation that returns (T, error) on nodes until success or all nodes are exhausted
func RunWithNodeRetry[T any](
	ctx context.Context,
	r RPCManagerInterface,
	operation func(*NodeConfig) (T, error),
	operationName string,
	mode int,
) (T, error) {
	var zeroValue T
	var err error

	triedNodes := make(map[int]bool)
	nodes := []NodeConfig{}
	if mode == GRPC_MODE {
		nodes, err = r.GetQueryNodes()
		if err != nil {
			return zeroValue, errorsmod.Wrapf(err, "error getting nodes")
		}
	} else if mode == RPC_MODE {
		nodes, err = r.GetTxNodes()
		if err != nil {
			return zeroValue, errorsmod.Wrapf(err, "error getting nodes")
		}
	} else {
		return zeroValue, errorsmod.Wrapf(errors.New("invalid server mode"), "invalid mode: %d, can be GRPC_MODE: %d or RPC_MODE: %d", mode, GRPC_MODE, RPC_MODE)
	}

	totalNodes := len(nodes)
	// Force change of initial node
	if mode == GRPC_MODE {
		r.SwitchToNextQueryNode()
	} else if mode == RPC_MODE {
		r.SwitchToNextTxNode()
	}

	for attempts := 0; attempts < totalNodes; attempts++ {
		var currentNode *NodeConfig
		var currentIdx int
		if mode == GRPC_MODE {
			currentNode = r.GetCurrentQueryNode()
			currentIdx = r.GetCurrentQueryIndex() // We can use the attempt number as the index
		} else if mode == RPC_MODE {
			currentNode = r.GetCurrentTxNode()
			currentIdx = r.GetCurrentTxIndex() // We can use the attempt number as the index
		} else {
			return zeroValue, errorsmod.Wrapf(errors.New("invalid server mode"), "invalid mode: %d, can be GRPC_MODE: %d or RPC_MODE: %d", mode, GRPC_MODE, RPC_MODE)
		}

		// Skip if we've already tried this node
		if triedNodes[currentIdx] {
			if mode == RPC_MODE {
				r.SwitchToNextQueryNode()
			} else {
				r.SwitchToNextTxNode()
			}
			continue
		}

		// Mark this node as tried
		triedNodes[currentIdx] = true

		// Attempt operation on current node - if no error, return result
		result, err := operation(currentNode)
		if err == nil {
			return result, nil
		}

		// If it's a node switching error, switch to next node and continue
		if IsErrorSwitchingNode(err) {
			log.Warn().
				Err(err).
				Str("rpc", currentNode.ServerAddress).
				Int("idx", currentIdx). // Changed from node address to index
				Str("operation", operationName).
				Msg("Error - Switching to next node")
			if mode == GRPC_MODE {
				r.SwitchToNextQueryNode()
			} else {
				r.SwitchToNextTxNode()
			}
			continue
		}

		// For any other error, return it immediately without switching to next node
		return zeroValue, errorsmod.Wrapf(err, "error during %s", operationName)
	}

	return zeroValue, errorsmod.Wrapf(ErrAllNodesExhausted,
		"tried %d nodes during %s", totalNodes, operationName)
}

func validateNodeURIs(nodes []string) error {
	for _, node := range nodes {
		// Validate URI
		if node != "" {
			_, err := url.ParseRequestURI(node)
			if err != nil {
				return fmt.Errorf("invalid RPC URL %s: %w", node, err)
			}
		}
	}
	return nil
}
