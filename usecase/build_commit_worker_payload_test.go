package usecase

// import (
// 	"allora_offchain_node/lib"
// 	"testing"

// 	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// )

// func (suite *UseCaseSuite) SetupTest() {
// 	// Any setup needed for each test
// }

// func TestComputeWorkerBundle(t *testing.T) {
// 	workerOptions := map[string]string{
// 		"InferenceEndpoint": "http://source:8000/inference/{Token}",
// 		"Token":             "ETH",
// 	}

// 	tests := []struct {
// 		name             string
// 		workerConfig     lib.WorkerConfig
// 		mockSetup        func(*MockAlloraAdapter)
// 		expectedResponse lib.WorkerResponse
// 		expectError      bool
// 		errorContains    string
// 	}{
// 		{
// 			name: "Happy path - valid prediction",
// 			workerConfig: lib.WorkerConfig{
// 				TopicId:                 emissionstypes.TopicId(1),
// 				InferenceEntrypointName: "api-worker-reputer",
// 				ForecastEntrypointName:  "api-worker-reputer",
// 				InferenceEntrypoint:     nil, // Will be set in the test
// 				ForecastEntrypoint:      nil, // Will be set in the test
// 				Parameters:              workerOptions,
// 				LoopSeconds:             10,
// 			},
// 			mockSetup: func(m *MockAlloraAdapter) {
// 				m.On("CalcInfer", mock.AnythingOfType("lib.WorkerConfig"), workerOptions).Return("9.5", nil)
// 				m.On("Forecast", mock.AnythingOfType("lib.WorkerConfig"), workerOptions).Return([]lib.NodeValue{{Value: "9.7", Woker: 1234567890}}, nil)
// 			},
// 			expectedResponse: lib.WorkerResponse{
// 				InfererValue:     "9.5",
// 				ForecasterValues: []lib.NodeValue{{Value: "9.7", Worker: "worker1"}},
// 			},
// 			expectError: false,
// 		},
// 		// Add more test cases here
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			mockAdapter := NewMockAlloraAdapter(t)
// 			tt.mockSetup(mockAdapter)
// 			tt.workerConfig.InferenceEntrypoint = mockAdapter
// 			tt.workerConfig.ForecastEntrypoint = mockAdapter

// 			response, err := suite.CalcInference(tt.workerConfig)

// 			if tt.expectError {
// 				assert.Error(t, err)
// 				assert.Contains(t, err.Error(), tt.errorContains)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, tt.expectedResponse.InfererValue, response.InfererValue)
// 				assert.Equal(t, tt.expectedResponse.ForecasterValues, response.ForecasterValues)
// 			}

// 			mockAdapter.AssertExpectations(t)
// 		})
// 	}
// }

// func TestSignWorkerValueBundle(t *testing.T) {
// 	// Implement test for SignWorkerValueBundle
// }

// func TestBuildCommitWorkerPayload(t *testing.T) {
// 	// Implement test for BuildCommitWorkerPayload
// }

// // Add more test functions as needed
