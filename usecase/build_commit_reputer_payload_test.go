package usecase

import (
	"allora_offchain_node/lib"
	"errors"
	"testing"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// type MockAlloraAdapter struct {
// 	mock.Mock
// }

// func (m *MockAlloraAdapter) Name() string {
// 	args := m.Called()
// 	return args.String(0)
// }

// func (m *MockAlloraAdapter) CalcInference(config lib.WorkerConfig, timestamp int64) (string, error) {
// 	args := m.Called(config, timestamp)
// 	return args.String(0), args.Error(1)
// }

// func (m *MockAlloraAdapter) CalcForecast(config lib.WorkerConfig, timestamp int64) ([]lib.NodeValue, error) {
// 	args := m.Called(config, timestamp)
// 	return args.Get(0).([]lib.NodeValue), args.Error(1)
// }

// func (m *MockAlloraAdapter) SourceTruth(config lib.ReputerConfig, timestamp int64) (lib.Truth, error) {
// 	args := m.Called(config, timestamp)
// 	return args.Get(0).(lib.Truth), args.Error(1)
// }

// func (m *MockAlloraAdapter) LossFunction(sourceTruth string, inferenceValue string) (string, error) {
// 	args := m.Called(sourceTruth, inferenceValue)
// 	return args.String(0), args.Error(1)
// }

// func (m *MockAlloraAdapter) CanInfer() bool {
// 	args := m.Called()
// 	return args.Bool(0)
// }

// func (m *MockAlloraAdapter) CanForecast() bool {
// 	args := m.Called()
// 	return args.Bool(0)
// }

// func (m *MockAlloraAdapter) CanSourceTruthAndComputeLoss() bool {
// 	args := m.Called()
// 	return args.Bool(0)
// }

func TestComputeLossBundle(t *testing.T) {
	tests := []struct {
		name                string
		sourceTruth         string
		valueBundle         *emissionstypes.ValueBundle
		reputerConfig       lib.ReputerConfig
		expectedLossStrings map[string]string
		mockSetup           func(*MockAlloraAdapter)
		expectError         bool
		errorContains       string
	}{
		{
			name:        "Happy path - all positive values",
			sourceTruth: "10.0",
			valueBundle: func() *emissionstypes.ValueBundle {
				combined, _ := alloraMath.NewDecFromString("9.5")
				naive, _ := alloraMath.NewDecFromString("9.0")
				inferer, _ := alloraMath.NewDecFromString("9.7")
				forecaster, _ := alloraMath.NewDecFromString("9.8")
				return &emissionstypes.ValueBundle{
					CombinedValue: combined,
					NaiveValue:    naive,
					InfererValues: []*emissionstypes.WorkerAttributedValue{
						{Value: inferer},
					},
					ForecasterValues: []*emissionstypes.WorkerAttributedValue{
						{Value: forecaster},
					},
				}
			}(),
			reputerConfig: lib.ReputerConfig{AllowsNegativeValue: true},
			expectedLossStrings: map[string]string{
				"CombinedValue":    "0.25",
				"NaiveValue":       "1.00",
				"InfererValues":    "0.09",
				"ForecasterValues": "0.04",
			},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", "10.0", "9.5").Return("0.25", nil)
				m.On("LossFunction", "10.0", "9.0").Return("1.00", nil)
				m.On("LossFunction", "10.0", "9.7").Return("0.09", nil)
				m.On("LossFunction", "10.0", "9.8").Return("0.04", nil)
			},
			expectError: false,
		},
		{
			name:        "Error in LossFunction",
			sourceTruth: "10.0",
			valueBundle: func() *emissionstypes.ValueBundle {
				combined, _ := alloraMath.NewDecFromString("9.5")
				return &emissionstypes.ValueBundle{
					CombinedValue: combined,
				}
			}(),
			reputerConfig: lib.ReputerConfig{AllowsNegativeValue: true},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", mock.Anything, mock.Anything).Return("", errors.New("loss function error"))
			},
			expectError:   true,
			errorContains: "error computing loss for combined value",
		},
		{
			name:        "Invalid loss value",
			sourceTruth: "10.0",
			valueBundle: func() *emissionstypes.ValueBundle {
				combined, _ := alloraMath.NewDecFromString("9.5")
				return &emissionstypes.ValueBundle{
					CombinedValue: combined,
				}
			}(),
			reputerConfig: lib.ReputerConfig{AllowsNegativeValue: true},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", mock.Anything, mock.Anything).Return("invalid", nil)
			},
			expectError:   true,
			errorContains: "error parsing loss",
		},
		{
			name:          "Nil ValueBundle",
			sourceTruth:   "10.0",
			valueBundle:   nil,
			reputerConfig: lib.ReputerConfig{AllowsNegativeValue: true},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "nil ValueBundle",
		},
		{
			name:          "Empty ValueBundle",
			sourceTruth:   "10.0",
			valueBundle:   &emissionstypes.ValueBundle{},
			reputerConfig: lib.ReputerConfig{AllowsNegativeValue: true},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "empty ValueBundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := ReturnBasicMockAlloraAdapter()
			tt.mockSetup(mockAdapter)
			tt.reputerConfig.ReputerEntrypoint = mockAdapter

			suite := &UseCaseSuite{}
			result, err := suite.ComputeLossBundle(tt.sourceTruth, tt.valueBundle, tt.reputerConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLossStrings["CombinedValue"], result.CombinedValue.String(), "Mismatch for CombinedValue")
				assert.Equal(t, tt.expectedLossStrings["NaiveValue"], result.NaiveValue.String(), "Mismatch for NaiveValue")
				for i, inferer := range result.InfererValues {
					assert.Equal(t, tt.expectedLossStrings["InfererValues"], inferer.Value.String(), "Mismatch for InfererValue %d", i)
				}
				for i, forecaster := range result.ForecasterValues {
					assert.Equal(t, tt.expectedLossStrings["ForecasterValues"], forecaster.Value.String(), "Mismatch for ForecasterValue %d", i)
				}
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}
