package usecase

import (
	"allora_offchain_node/lib"

	"github.com/stretchr/testify/mock"
)

type MockAlloraAdapter struct {
	mock.Mock
}

func (m *MockAlloraAdapter) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAlloraAdapter) CalcInference(config lib.WorkerConfig, timestamp int64) (string, error) {
	args := m.Called(config, timestamp)
	return args.String(0), args.Error(1)
}

func (m *MockAlloraAdapter) CalcForecast(config lib.WorkerConfig, timestamp int64) ([]lib.NodeValue, error) {
	args := m.Called(config, timestamp)
	return args.Get(0).([]lib.NodeValue), args.Error(1)
}

func (m *MockAlloraAdapter) SourceTruth(config lib.ReputerConfig, timestamp int64) (lib.Truth, error) {
	args := m.Called(config, timestamp)
	return args.Get(0).(lib.Truth), args.Error(1)
}

func (m *MockAlloraAdapter) LossFunction(sourceTruth string, inferenceValue string) (string, error) {
	args := m.Called(sourceTruth, inferenceValue)
	return args.String(0), args.Error(1)
}

func (m *MockAlloraAdapter) CanInfer() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockAlloraAdapter) CanForecast() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockAlloraAdapter) CanSourceTruthAndComputeLoss() bool {
	args := m.Called()
	return args.Bool(0)
}

func NewMockAlloraAdapter() *MockAlloraAdapter {
	return &MockAlloraAdapter{}
}

func ReturnBasicMockAlloraAdapter() *MockAlloraAdapter {
	return NewMockAlloraAdapter()
}
