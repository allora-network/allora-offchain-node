package usecase

import (
	"errors"
	"testing"

	"allora_offchain_node/lib"
	"allora_offchain_node/metrics"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (suite *UseCaseSuite) SetupTest() {
	// Any setup needed for each test
}

func TestComputeWorkerBundle(t *testing.T) {
	workerOptions := map[string]string{
		"InferenceEndpoint": "http://source:8000/inference/{Token}",
		"Token":             "ETH",
	}

	tests := []struct {
		name             string
		workerConfig     lib.WorkerResponse
		mockSetup        func(*MockAlloraAdapter)
		expectedResponse emissionstypes.InferenceForecastBundle
		expectError      bool
		errorContains    string
		address          string
	}{
		{
			name: "Happy path - valid prediction",
			workerConfig: lib.WorkerResponse{
				WorkerConfig: lib.WorkerConfig{
					TopicId:                 emissionstypes.TopicId(1),
					InferenceEntrypointName: "apiAdapter",
					ForecastEntrypointName:  "apiAdapter",
					InferenceEntrypoint:     nil, // Will be set in the test
					ForecastEntrypoint:      nil, // Will be set in the test
					Parameters:              workerOptions,
				},
				InfererValues: []lib.LabeledValue{
					{Label: "A", Value: "9.5"},
					{Label: "B", Value: "7.6"},
				},
				ForecasterValues: []lib.NodeValue{
					{Value: "9.7", Worker: "worker1"},
				},
			},
			mockSetup: func(m *MockAlloraAdapter) {
			},
			expectedResponse: emissionstypes.InferenceForecastBundle{
				Inference: &emissionstypes.Inference{ //nolint:exhaustruct
					TopicId:     uint64(1),
					BlockHeight: 1,
					Inferer:     "worker1",
					Values:      alloraMath.DecArray{alloraMath.MustNewDecFromString("9.5"), alloraMath.MustNewDecFromString("7.6")},
				},
				Forecast: &emissionstypes.Forecast{ //nolint:exhaustruct
					TopicId:     uint64(1),
					BlockHeight: 1,
					Forecaster:  "worker1",
					ForecastElements: []*emissionstypes.ForecastElement{
						{
							Inferer: "worker1",
							Value:   alloraMath.MustNewDecFromString("9.7"),
						},
					},
				},
			},
			expectError:   false,
			errorContains: "",
			address:       "worker1",
		},
		{ //nolint:exhaustruct
			name: "Invalid inference value",
			workerConfig: lib.WorkerResponse{
				WorkerConfig: lib.WorkerConfig{
					TopicId:                 emissionstypes.TopicId(1),
					InferenceEntrypointName: "apiAdapter",
					ForecastEntrypointName:  "apiAdapter",
					InferenceEntrypoint:     nil,
					ForecastEntrypoint:      nil,
					Parameters:              workerOptions,
				},
				InfererValue: "invalid",
				ForecasterValues: []lib.NodeValue{
					{Value: "9.7", Worker: "worker1"},
				},
			},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "invalid decimal string",
			address:       "worker1",
		},
		{ //nolint:exhaustruct
			name: "Invalid forecast value",
			workerConfig: lib.WorkerResponse{
				WorkerConfig: lib.WorkerConfig{
					TopicId:                 emissionstypes.TopicId(1),
					InferenceEntrypointName: "apiAdapter",
					ForecastEntrypointName:  "apiAdapter",
					InferenceEntrypoint:     nil,
					ForecastEntrypoint:      nil,
					Parameters:              workerOptions,
				},
				InfererValue: "9.5",
				ForecasterValues: []lib.NodeValue{
					{Value: "invalid", Worker: "worker1"},
				},
			},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "invalid decimal string",
			address:       "worker1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := NewMockAlloraAdapter()
			tt.mockSetup(mockAdapter)
			tt.workerConfig.InferenceEntrypoint = mockAdapter
			tt.workerConfig.ForecastEntrypoint = mockAdapter

			// Create mock wallet
			mockWallet := &lib.Wallet{ //nolint:exhaustruct
				Address: tt.address,
				// Other wallet fields are not required for this test
			}

			// Replace ConnectionManager creation with mock
			mockConnectionManager := &lib.MockConnectionManager{} //nolint:exhaustruct
			mockNodeConfig := &lib.NodeConfig{}                   //nolint:exhaustruct

			// Add mock expectations
			mockConnectionManager.On("GetCurrentQueryNode").Return(mockNodeConfig)
			mockConnectionManager.On("GetCurrentTxNode").Return(mockNodeConfig)
			mockConnectionManager.On("GetWallet").Return(mockWallet, nil)

			suite := &UseCaseSuite{ConnectionManager: mockConnectionManager} //nolint:exhaustruct

			response, err := suite.BuildWorkerPayload(tt.workerConfig, 1)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResponse.Inference.BlockHeight, response.Inference.BlockHeight)
				assert.Equal(t, tt.expectedResponse.Inference.Inferer, response.Inference.Inferer)
				assert.Equal(t, tt.expectedResponse.Inference.TopicId, response.Inference.TopicId)
				infererValuesDec := make([]alloraMath.Dec, len(response.Inference.Values))
				for i := range response.Inference.Values {
					infererValuesDec[i] = response.Inference.Values[i].Value.ToDec()
				}
				assert.Equal(t, tt.expectedResponse.Inference.Values, infererValuesDec)
				assert.Equal(t, tt.expectedResponse.Forecast.BlockHeight, response.Forecast.BlockHeight)
				assert.Equal(t, tt.expectedResponse.Forecast.Forecaster, response.Forecast.Forecaster)
				assert.Equal(t, tt.expectedResponse.Forecast.TopicId, response.Forecast.TopicId)
				assert.Equal(t, len(tt.expectedResponse.Forecast.ForecastElements), len(response.Forecast.ForecastElements))
				// element value matching
				for _, expectedElement := range tt.expectedResponse.Forecast.ForecastElements {
					found := false
					for _, actualElement := range response.Forecast.ForecastElements {
						actualElementDec := actualElement.Value.ToDec()
						require.NoError(t, err)
						if expectedElement.Inferer == actualElement.Inferer && expectedElement.Value.Equal(actualElementDec) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected forecast element not found: %v", expectedElement)
				}
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}

// TestBuildWorkerPayloadScalarAndMultiLabel exercises BuildWorkerPayload across
// both supported use-cases: the legacy scalar inference (InfererValue) and the
// new multi-label / vector inference (InfererValues, e.g. classification), plus
// the inference-only, forecast-only and empty combinations.
//
//nolint:exhaustruct
func TestBuildWorkerPayloadScalarAndMultiLabel(t *testing.T) {
	const address = "worker1"

	workerConfig := func() lib.WorkerConfig {
		return lib.WorkerConfig{
			TopicId:                 emissionstypes.TopicId(1),
			InferenceEntrypointName: "apiAdapter",
			ForecastEntrypointName:  "apiAdapter",
			Parameters: map[string]string{
				"InferenceEndpoint": "http://source:8000/inference/{Token}",
				"Token":             "ETH",
			},
		}
	}

	// equalDec asserts that a BoundedExp40Dec-derived Dec equals the expected
	// decimal string, mirroring how the other worker assertions compare values.
	equalDec := func(t *testing.T, expected string, actual alloraMath.Dec) {
		t.Helper()
		assert.True(t, alloraMath.MustNewDecFromString(expected).Equal(actual),
			"expected %s, got %s", expected, actual.String())
	}

	tests := []struct {
		name           string
		workerResponse lib.WorkerResponse
		expectError    bool
		errorContains  string
		assertResult   func(*testing.T, emissionstypes.InputInferenceForecastBundle)
	}{
		{
			name: "scalar inference (legacy) with forecast",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
				InfererValue: "9.5",
				ForecasterValues: []lib.NodeValue{
					{Worker: "fc1", Value: "1.2"},
				},
			},
			assertResult: func(t *testing.T, b emissionstypes.InputInferenceForecastBundle) {
				require.NotNil(t, b.Inference)
				assert.Equal(t, uint64(1), b.Inference.TopicId)
				assert.Equal(t, int64(1), b.Inference.BlockHeight)
				assert.Equal(t, address, b.Inference.Inferer)
				equalDec(t, "9.5", b.Inference.Value.ToDec())
				// Scalar path must not populate the multi-label Values field.
				assert.Empty(t, b.Inference.Values)
				require.NotNil(t, b.Forecast)
				assert.Equal(t, address, b.Forecast.Forecaster)
				require.Len(t, b.Forecast.ForecastElements, 1)
				assert.Equal(t, "fc1", b.Forecast.ForecastElements[0].Inferer)
				equalDec(t, "1.2", b.Forecast.ForecastElements[0].Value.ToDec())
			},
		},
		{
			name: "multi-label classification inference - three classes, no forecast",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
				InfererValues: []lib.LabeledValue{
					{Label: "UP", Value: "0.3"},
					{Label: "MID", Value: "0.5"},
					{Label: "DOWN", Value: "0.2"},
				},
			},
			assertResult: func(t *testing.T, b emissionstypes.InputInferenceForecastBundle) {
				require.NotNil(t, b.Inference)
				assert.Equal(t, address, b.Inference.Inferer)
				require.Len(t, b.Inference.Values, 3)
				assert.Equal(t, "UP", b.Inference.Values[0].Label)
				equalDec(t, "0.3", b.Inference.Values[0].Value.ToDec())
				assert.Equal(t, "MID", b.Inference.Values[1].Label)
				equalDec(t, "0.5", b.Inference.Values[1].Value.ToDec())
				assert.Equal(t, "DOWN", b.Inference.Values[2].Label)
				equalDec(t, "0.2", b.Inference.Values[2].Value.ToDec())
				// No forecast values were provided.
				assert.Nil(t, b.Forecast)
			},
		},
		{
			name: "forecast only - no inference",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
				ForecasterValues: []lib.NodeValue{
					{Worker: "fc1", Value: "1.2"},
					{Worker: "fc2", Value: "3.4"},
				},
			},
			assertResult: func(t *testing.T, b emissionstypes.InputInferenceForecastBundle) {
				assert.Nil(t, b.Inference)
				require.NotNil(t, b.Forecast)
				require.Len(t, b.Forecast.ForecastElements, 2)
				assert.Equal(t, "fc1", b.Forecast.ForecastElements[0].Inferer)
				assert.Equal(t, "fc2", b.Forecast.ForecastElements[1].Inferer)
			},
		},
		{
			name: "neither inference nor forecast - empty bundle",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
			},
			assertResult: func(t *testing.T, b emissionstypes.InputInferenceForecastBundle) {
				assert.Nil(t, b.Inference)
				assert.Nil(t, b.Forecast)
			},
		},
		{
			name: "invalid multi-label inference value",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
				InfererValues: []lib.LabeledValue{
					{Label: "UP", Value: "0.3"},
					{Label: "MID", Value: "invalid"},
				},
			},
			expectError:   true,
			errorContains: "invalid decimal string",
		},
		{
			name: "invalid scalar inference value",
			workerResponse: lib.WorkerResponse{
				WorkerConfig: workerConfig(),
				InfererValue: "invalid",
			},
			expectError:   true,
			errorContains: "invalid decimal string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWallet := &lib.Wallet{Address: address}

			mockConnectionManager := &lib.MockConnectionManager{}
			mockConnectionManager.On("GetWallet").Return(mockWallet, nil)

			suite := &UseCaseSuite{ConnectionManager: mockConnectionManager}

			bundle, err := suite.BuildWorkerPayload(tt.workerResponse, 1)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}
			require.NoError(t, err)
			if tt.assertResult != nil {
				tt.assertResult(t, bundle)
			}
		})
	}
}

// TestGetWorkerResponseDispatch verifies that getWorkerResponse dispatches to the
// correct inference entrypoint method depending on whether the worker is configured
// for multi-label inference (LabeledInferenceEndpoint present) or scalar inference.
// The mock fails the test if an unexpected method is called, so each case implicitly
// asserts the other path is NOT taken.
//
//nolint:exhaustruct
func TestGetWorkerResponseDispatch(t *testing.T) {
	const (
		address     = "worker1"
		blockHeight = int64(10)
	)

	tests := []struct {
		name          string
		parameters    map[string]string
		withForecast  bool
		mockSetup     func(*MockAlloraAdapter)
		expectError   bool
		errorContains string
		assertResult  func(*testing.T, lib.WorkerResponse)
	}{
		{
			name:       "scalar inference dispatch - no LabeledInferenceEndpoint",
			parameters: map[string]string{"InferenceEndpoint": "http://x/inference"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("CalcInference", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).Return("0.5", nil).Once()
			},
			assertResult: func(t *testing.T, r lib.WorkerResponse) {
				assert.Equal(t, "0.5", r.InfererValue)
				assert.Empty(t, r.InfererValues)
			},
		},
		{
			name:       "multi-label inference dispatch - LabeledInferenceEndpoint present",
			parameters: map[string]string{"LabeledInferenceEndpoint": "http://x/labeled-inference"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("CalcLabeledInference", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).
					Return([]lib.LabeledValue{{Label: "UP", Value: "0.3"}, {Label: "DOWN", Value: "0.7"}}, nil).Once()
			},
			assertResult: func(t *testing.T, r lib.WorkerResponse) {
				assert.Empty(t, r.InfererValue)
				require.Len(t, r.InfererValues, 2)
				assert.Equal(t, "UP", r.InfererValues[0].Label)
				assert.Equal(t, "0.3", r.InfererValues[0].Value)
				assert.Equal(t, "DOWN", r.InfererValues[1].Label)
				assert.Equal(t, "0.7", r.InfererValues[1].Value)
			},
		},
		{
			name:         "multi-label inference plus forecast",
			parameters:   map[string]string{"LabeledInferenceEndpoint": "http://x/labeled-inference"},
			withForecast: true,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("CalcLabeledInference", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).
					Return([]lib.LabeledValue{{Label: "UP", Value: "0.3"}}, nil).Once()
				m.On("CalcForecast", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).
					Return([]lib.NodeValue{{Worker: "w2", Value: "0.4"}}, nil).Once()
			},
			assertResult: func(t *testing.T, r lib.WorkerResponse) {
				require.Len(t, r.InfererValues, 1)
				require.Len(t, r.ForecasterValues, 1)
				assert.Equal(t, "w2", r.ForecasterValues[0].Worker)
			},
		},
		{
			name:       "scalar inference error propagates",
			parameters: map[string]string{},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("CalcInference", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).
					Return("", errors.New("inference endpoint down")).Once()
			},
			expectError:   true,
			errorContains: "Error computing inference",
		},
		{
			name:       "labeled inference error propagates",
			parameters: map[string]string{"LabeledInferenceEndpoint": "http://x/labeled-inference"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("CalcLabeledInference", mock.AnythingOfType("lib.WorkerConfig"), blockHeight).
					Return([]lib.LabeledValue(nil), errors.New("labeled endpoint down")).Once()
			},
			expectError:   true,
			errorContains: "Error computing labeled inference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := NewMockAlloraAdapter()
			tt.mockSetup(mockAdapter)

			worker := lib.WorkerConfig{
				TopicId:             emissionstypes.TopicId(1),
				InferenceEntrypoint: mockAdapter,
				Parameters:          tt.parameters,
			}
			if tt.withForecast {
				worker.ForecastEntrypoint = mockAdapter
			}

			suite := &UseCaseSuite{Metrics: &metrics.Metrics{}}

			resp, err := suite.getWorkerResponse(worker, blockHeight, address)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.assertResult != nil {
					tt.assertResult(t, resp)
				}
			}
			mockAdapter.AssertExpectations(t)
		})
	}
}

// Add more test functions as needed
