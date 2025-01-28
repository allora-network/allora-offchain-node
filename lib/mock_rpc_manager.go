package lib

import (
	"context"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"
)

type MockRPCManager struct {
	mock.Mock
}

func (m *MockRPCManager) GetCurrentQueryNode() *NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) GetCurrentTxNode() *NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToNextQueryNode() *NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToNextTxNode() *NodeConfig {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
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

func (m *MockRPCManager) GetQueryNodes() ([]NodeConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if nodes, ok := args.Get(0).([]NodeConfig); ok {
		return nodes, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRPCManager) GetTxNodes() ([]NodeConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if nodes, ok := args.Get(0).([]NodeConfig); ok {
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

func (m *MockRPCManager) SendDataWithRetry(ctx context.Context, req sdk.Msg, infoMsg string, timeoutHeight uint64) (*coretypes.ResultBroadcastTx, error) {
	args := m.Called(ctx, req, infoMsg, timeoutHeight)
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

func (m *MockRPCManager) SwitchToQueryNode(index int) *NodeConfig {
	args := m.Called(index)
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) SwitchToTxNode(index int) *NodeConfig {
	args := m.Called(index)
	if args.Get(0) == nil {
		return nil
	}
	if node, ok := args.Get(0).(*NodeConfig); ok {
		return node
	}
	return nil
}

func (m *MockRPCManager) Close() error {
	return nil
}

func (m *MockRPCManager) GetWallet() (*Wallet, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if wallet, ok := args.Get(0).(*Wallet); ok {
		return wallet, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRPCManager) GetWalletConfig() (*WalletConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if walletConfig, ok := args.Get(0).(*WalletConfig); ok {
		return walletConfig, args.Error(1)
	}
	return nil, args.Error(1)
}
