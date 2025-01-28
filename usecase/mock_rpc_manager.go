package usecase

import (
	"allora_offchain_node/lib"
	"context"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"
)

type MockRPCManager struct {
	mock.Mock
}

func (m *MockRPCManager) GetCurrentQueryNode() *lib.NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) GetCurrentTxNode() *lib.NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToNextQueryNode() *lib.NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToNextTxNode() *lib.NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) GetStats() (int, map[int]int) {
	args := m.Called()
	failures, ok := args.Get(1).(map[int]int)
	if !ok {
		return args.Int(0), make(map[int]int)
	}
	return args.Int(0), failures
}

func (m *MockRPCManager) GetQueryNodes() ([]lib.NodeConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if nodes, ok := args.Get(0).([]lib.NodeConfig); ok {
		return nodes, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRPCManager) GetTxNodes() ([]lib.NodeConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if nodes, ok := args.Get(0).([]lib.NodeConfig); ok {
		return nodes, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRPCManager) SendDataWithNodeRetry(ctx context.Context, msg sdk.Msg, timeoutHeight uint64, operationName string) (*coretypes.ResultBroadcastTx, error) {
	args := m.Called(ctx, msg, timeoutHeight, operationName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if response, ok := args.Get(0).(*coretypes.ResultBroadcastTx); ok {
		return response, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRPCManager) GetCurrentQueryIndex() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRPCManager) GetCurrentTxIndex() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRPCManager) SwitchToQueryNode(index int) *lib.NodeConfig {
	args := m.Called(index)
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToTxNode(index int) *lib.NodeConfig {
	args := m.Called(index)
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*lib.NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) Close() error {
	return nil
}
