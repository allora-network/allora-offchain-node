package usecase

import (
	"allora_offchain_node/lib"
	"context"
	"fmt"
	"net/url"
	"sync"

	errorsmod "cosmossdk.io/errors"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
	"github.com/rs/zerolog/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type RPCManagerInterface interface {
	GetCurrentNode() *lib.NodeConfig
	GetCurrentIndex() int
	SwitchToNextNode() *lib.NodeConfig
	GetStats() (int, map[int]int)
	SendDataWithNodeRetry(ctx context.Context, msg sdk.Msg, timeoutHeight uint64, operationName string) (*cosmosclient.Response, error)
	GetNodes() ([]lib.NodeConfig, error)
}

type RPCManager struct {
	nodes      []lib.NodeConfig
	currentIdx int
	mu         sync.RWMutex
	stats      RPCStats
}

type RPCStats struct {
	nodeFailures map[int]int
	nodeSwitches int
	mu           sync.RWMutex
}

func (r *RPCManager) GetNodes() ([]lib.NodeConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nodes, nil
}

func (r *RPCManager) GetCurrentIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentIdx
}

func (r *RPCManager) GetCurrentNode() *lib.NodeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return &r.nodes[r.currentIdx]
}

// newRPCManager is now private, used internally by InitRPCManager
func NewRPCManager(userConfig lib.UserConfig) (*RPCManager, error) {
	if len(userConfig.Wallet.NodeRPCs) == 0 {
		return nil, fmt.Errorf("no RPC nodes provided")
	}

	// validate all urls are correct
	err := validateNodes(userConfig.Wallet.NodeRPCs)
	if err != nil {
		return nil, err
	}

	// Load here the nodeconfigs
	var nodes []lib.NodeConfig
	for _, rpc := range userConfig.Wallet.NodeRPCs {
		log.Info().Str("rpc", rpc).Msg("Initializing rpc")
		nodeConfig, err := userConfig.GenerateNodeConfig(rpc)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *nodeConfig)
	}

	return &RPCManager{ // nolint: exhaustruct
		nodes:      nodes,
		currentIdx: 0,
		stats: RPCStats{ // nolint: exhaustruct
			nodeFailures: make(map[int]int),
		},
		// no need to init mu
	}, nil
}

func validateNodes(nodes []string) error {
	for _, node := range nodes {
		// Validate URL
		_, err := url.ParseRequestURI(node)
		if err != nil {
			return fmt.Errorf("invalid RPC URL %s: %w", node, err)
		}
	}
	return nil
}

// SwitchToNextNode switches to the next node in the list.
// Node change is persistent, so it will be used again in the next call
func (r *RPCManager) SwitchToNextNode() *lib.NodeConfig {
	log.Info().Msg("Switching to next RPC node")
	r.mu.Lock()
	defer r.mu.Unlock()

	oldIndex := r.currentIdx
	r.currentIdx = (r.currentIdx + 1) % len(r.nodes)
	newNode := r.nodes[r.currentIdx]

	// Update stats
	r.stats.mu.Lock()
	r.stats.nodeFailures[oldIndex]++
	r.stats.nodeSwitches++
	r.stats.mu.Unlock()

	log.Warn().
		Int("from_node", oldIndex).
		Int("to_node", r.currentIdx).
		Int("total_switches", r.stats.nodeSwitches).
		Int("node_failures", r.stats.nodeFailures[oldIndex]).
		Msg("Switching to next RPC node")

	return &newNode
}

func (r *RPCManager) GetStats() (int, map[int]int) {
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()

	// Create a copy of the stats to return
	failuresCopy := make(map[int]int)
	for k, v := range r.stats.nodeFailures {
		failuresCopy[k] = v
	}

	return r.stats.nodeSwitches, failuresCopy
}

// SendDataWithNodeRetry attempts to send data to the chain, switching nodes if necessary
func (r *RPCManager) SendDataWithNodeRetry(
	ctx context.Context,
	msg sdk.Msg,
	timeoutHeight uint64,
	operationName string,
) (*cosmosclient.Response, error) {
	// Track which nodes we've tried
	triedNodes := make(map[int]bool)
	totalNodes := len(r.nodes)

	for attempts := 0; attempts < totalNodes; attempts++ {
		currentNode := r.GetCurrentNode()
		currentIdx := r.GetCurrentIndex()
		log.Debug().Str("rpc", r.GetCurrentNode().Wallet.NodeRPCs[currentIdx]).Msg("Attempting with current node")

		// Skip if we've already tried this node
		if triedNodes[currentIdx] {
			log.Debug().Int("idx", currentIdx).
				Str("rpc", r.GetCurrentNode().Wallet.NodeRPCs[currentIdx]).
				Str("operation", operationName).
				Msg("Already tried this node, switching to next")
			r.SwitchToNextNode()
			continue
		}

		// Mark this node as tried
		triedNodes[currentIdx] = true

		// Attempt to send data using existing retry mechanism
		res, err := currentNode.SendDataWithRetry(ctx, msg, operationName, timeoutHeight)
		if err == nil {
			return res, nil
		}

		// If it's a node switching error, switch to next node and continue
		if lib.IsErrorSwitchingNode(err) {
			log.Warn().
				Int("idx", currentIdx).
				Str("rpc", r.GetCurrentNode().Wallet.NodeRPCs[currentIdx]).
				Str("operation", operationName).
				Msg("Switching to next node")
			r.SwitchToNextNode()
			continue
		}

		// For any other error, return it immediately
		return nil, errorsmod.Wrapf(err, "error during %s", operationName)
	}

	// If we've tried all nodes and none worked
	return nil, errorsmod.Wrapf(ErrAllNodesExhausted,
		"tried %d nodes during %s", totalNodes, operationName)
}

// RunWithNodeRetry executes an operation that returns (T, error) on nodes until success or all nodes are exhausted
func RunWithNodeRetry[T any](
	ctx context.Context,
	r RPCManagerInterface,
	operation func(*lib.NodeConfig) (T, error),
	operationName string,
) (T, error) {
	var zeroValue T

	// Change to use indices instead of addresses
	triedNodes := make(map[int]bool)
	nodes, err := r.GetNodes()
	if err != nil {
		return zeroValue, errorsmod.Wrapf(err, "error getting nodes")
	}
	totalNodes := len(nodes)

	for attempts := 0; attempts < totalNodes; attempts++ {
		currentNode := r.GetCurrentNode()
		currentIdx := r.GetCurrentIndex() // We can use the attempt number as the index

		// Skip if we've already tried this node
		if triedNodes[currentIdx] {
			r.SwitchToNextNode()
			continue
		}

		// Mark this node as tried
		triedNodes[currentIdx] = true

		// Attempt operation on current node
		result, err := operation(currentNode)
		if err == nil {
			return result, nil
		}

		// If it's a node switching error, switch to next node and continue
		if lib.IsErrorSwitchingNode(err) {
			log.Warn().
				Int("idx", currentIdx). // Changed from node address to index
				Str("operation", operationName).
				Msg("Switching to next node")
			r.SwitchToNextNode()
			continue
		}

		// For any other error, return it immediately
		return zeroValue, errorsmod.Wrapf(err, "error during %s", operationName)
	}

	// If we've tried all nodes and none worked
	return zeroValue, errorsmod.Wrapf(ErrAllNodesExhausted,
		"tried %d nodes during %s", totalNodes, operationName)
}
