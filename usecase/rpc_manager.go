package usecase

import (
	"allora_offchain_node/lib"
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
	GetCurrentQueryNode() *lib.NodeConfig
	GetCurrentTxNode() *lib.NodeConfig
	GetCurrentQueryIndex() int
	GetCurrentTxIndex() int
	SwitchToNextQueryNode() *lib.NodeConfig
	SwitchToNextTxNode() *lib.NodeConfig
	SwitchToQueryNode(index int) *lib.NodeConfig
	SwitchToTxNode(index int) *lib.NodeConfig
	SendDataWithNodeRetry(ctx context.Context, msg sdk.Msg, timeoutHeight uint64, operationName string) (*coretypes.ResultBroadcastTx, error)
	GetQueryNodes() ([]lib.NodeConfig, error)
	GetTxNodes() ([]lib.NodeConfig, error)
	Close() error
}

const (
	GRPC_MODE int = 1
	RPC_MODE  int = 2
)

type RPCManager struct {
	queryNodes []lib.NodeConfig
	txNodes    []lib.NodeConfig
	currentIdx int
	mu         sync.RWMutex
	wallet     *lib.Wallet
	walletInit sync.Once
}

func NewRPCManager(ctx context.Context, userConfig lib.UserConfig) (*RPCManager, error) {
	if len(userConfig.Wallet.NodeRPCs) == 0 {
		return nil, fmt.Errorf("no GRPC/RPC nodes provided")
	}

	wallet, err := lib.NewWalletFromConfig(ctx, userConfig.Wallet)
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

	// Load here the nodeconfigs
	var queryNodes []lib.NodeConfig
	for _, grpc := range userConfig.Wallet.NodeGRPCs {
		log.Info().Str("grpc", grpc).Msg("Initializing grpc query nodes")
		nodeConfig, err := userConfig.GenerateNodeConfig(ctx, wallet, "", grpc)
		if err != nil {
			log.Error().Err(err).Str("grpc", grpc).Msg("Error generating node config, skipping GRPC node")
			continue
		}
		queryNodes = append(queryNodes, *nodeConfig)
	}

	var txNodes []lib.NodeConfig

	for _, rpc := range userConfig.Wallet.NodeRPCs {
		log.Info().Str("rpc", rpc).Msg("Initializing rpc tx nodes")
		nodeConfig, err := userConfig.GenerateNodeConfig(ctx, wallet, rpc, "")
		if err != nil {
			log.Error().Err(err).Str("rpc", rpc).Msg("Error generating node config, skipping RPC node")
			continue
		}
		txNodes = append(txNodes, *nodeConfig)
	}
	return &RPCManager{ // nolint: exhaustruct
		queryNodes: queryNodes,
		txNodes:    txNodes,
		currentIdx: 0,
		// no need to init mu
	}, nil
}

func (r *RPCManager) InitializeWallet(ctx context.Context, walletConfig lib.WalletConfig) error {
	var initErr error
	r.walletInit.Do(func() {
		wallet, err := lib.NewWalletFromConfig(ctx, walletConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize wallet: %w", err)
			return
		}
		r.wallet = wallet
	})
	return initErr
}

// GetWallet returns the wallet instance, returns error if wallet is not initialized
func (r *RPCManager) GetWallet() (*lib.Wallet, error) {
	if r.wallet == nil {
		return nil, fmt.Errorf("wallet not initialized")
	}
	return r.wallet, nil
}

func (r *RPCManager) GetQueryNodes() ([]lib.NodeConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.queryNodes, nil
}

func (r *RPCManager) GetTxNodes() ([]lib.NodeConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.txNodes, nil
}

func (r *RPCManager) GetCurrentQueryIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentIdx
}

func (r *RPCManager) GetCurrentTxIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentIdx
}

func (r *RPCManager) GetCurrentQueryNode() *lib.NodeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return &r.queryNodes[r.currentIdx]
}

func (r *RPCManager) GetCurrentTxNode() *lib.NodeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return &r.txNodes[r.currentIdx]
}

// internal function, switches to a node assuming a lock has been acquired
func (r *RPCManager) switchToNodeLocked(index int, nodes []lib.NodeConfig) *lib.NodeConfig {
	if len(nodes) == 1 {
		return &nodes[0]
	}
	oldIndex := r.currentIdx
	r.currentIdx = index

	log.Debug().
		Str("from", nodes[oldIndex].ServerAddress).
		Str("to", nodes[index].ServerAddress).
		Msg("Switch to next RPC node")

	return &nodes[index]
}

// SwitchToNextNode switches to the next node in the list.
// Node change is persistent, so it will be used again in the next call
func (r *RPCManager) SwitchToNextQueryNode() *lib.NodeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Get next node index, wrap around if necessary
	nextNode := (r.currentIdx + 1) % len(r.queryNodes)
	return r.switchToNodeLocked(nextNode, r.queryNodes)
}

func (r *RPCManager) SwitchToNextTxNode() *lib.NodeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Get next node index, wrap around if necessary
	nextNode := (r.currentIdx + 1) % len(r.txNodes)
	return r.switchToNodeLocked(nextNode, r.txNodes)
}

// Switches to a specific node, acquiring a lock
func (r *RPCManager) SwitchToQueryNode(index int) *lib.NodeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.switchToNodeLocked(index, r.queryNodes)
}

func (r *RPCManager) SwitchToTxNode(index int) *lib.NodeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.switchToNodeLocked(index, r.txNodes)
}

func (r *RPCManager) SendDataWithNodeRetry(
	ctx context.Context,
	msg sdk.Msg,
	timeoutHeight uint64,
	operationName string,
) (*coretypes.ResultBroadcastTx, error) {
	return RunWithNodeRetry(ctx, r, func(node *lib.NodeConfig) (*coretypes.ResultBroadcastTx, error) {
		return node.SendDataWithRetry(ctx, msg, operationName, timeoutHeight)
	}, operationName, RPC_MODE)
}

// RunWithNodeRetry executes an operation that returns (T, error) on nodes until success or all nodes are exhausted
func RunWithNodeRetry[T any](
	ctx context.Context,
	r RPCManagerInterface,
	operation func(*lib.NodeConfig) (T, error),
	operationName string,
	mode int,
) (T, error) {
	var zeroValue T
	var err error

	triedNodes := make(map[int]bool)
	nodes := []lib.NodeConfig{}
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
		var currentNode *lib.NodeConfig
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
		if lib.IsErrorSwitchingNode(err) {
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

func (r *RPCManager) Close() error {
	log.Info().Msg("Closing RPCManager")
	// Iterate through all nodes and close them
	for _, node := range r.queryNodes {
		if node.Chain.Client.GRPCClient != nil {
			err := node.Chain.Client.GRPCClient.Close()
			if err != nil {
				return err
			}
		}
		if node.Chain.Client.RPCClient != nil {
			err := node.Chain.Client.RPCClient.Stop()
			if err != nil {
				return err
			}
		}
	}

	for _, node := range r.txNodes {
		if node.Chain.Client.GRPCClient != nil {
			err := node.Chain.Client.GRPCClient.Close()
			if err != nil {
				return err
			}
		}
		if node.Chain.Client.RPCClient != nil {
			err := node.Chain.Client.RPCClient.Stop()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
