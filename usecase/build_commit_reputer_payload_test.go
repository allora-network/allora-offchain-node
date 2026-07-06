package usecase

import (
	"errors"
	"fmt"
	"testing"

	"allora_offchain_node/lib"
	"allora_offchain_node/metrics"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestComputeLossBundle exercises ComputeLossBundle across the single-label
// path (scalar LossFunction) and the multi-label path (LabeledLossFunction),
// and verifies the one-out / one-in fields including the OneOutInfererForecaster
// regroup-by-forecaster logic.
//
//nolint:exhaustruct
func TestComputeLossBundle(t *testing.T) {
	reputerOptions := map[string]string{
		"method": "sqe",
	}

	// Single-label (SINGLE arity) reputer config: scalar LossFunctionService set,
	// never-negative cached. computeLoss takes the scalar LossFunction branch.
	singleLabelConfig := lib.ReputerConfig{
		LossFunctionParameters: lib.LossFunctionParameters{
			LossMethodOptions:   reputerOptions,
			IsNeverNegative:     &[]bool{false}[0],
			LossFunctionService: "scalar-loss-svc",
		},
	}

	// Multi-label (MULTI arity) reputer config: LabeledLossFunctionService set,
	// never-negative cached. computeLoss takes the LabeledLossFunction branch.
	multiLabelConfig := lib.ReputerConfig{
		LossFunctionParameters: lib.LossFunctionParameters{
			LossMethodOptions:          reputerOptions,
			IsNeverNegative:            &[]bool{false}[0],
			LabeledLossFunctionService: "labeled-loss-svc",
		},
	}

	// Both services configured, never-negative cached. Used to prove that arity
	// (not config presence) decides the branch: a single-label topic with this
	// config must still use the scalar LossFunctionService.
	bothServicesConfig := lib.ReputerConfig{
		LossFunctionParameters: lib.LossFunctionParameters{
			LossMethodOptions:          reputerOptions,
			IsNeverNegative:            &[]bool{false}[0],
			LossFunctionService:        "scalar-loss-svc",
			LabeledLossFunctionService: "labeled-loss-svc",
		},
	}

	// Same as above but WITHOUT a cached IsNeverNegative, so computeLoss must
	// query IsLossFunctionNeverNegative against the relevant service endpoint
	// (scalar LossFunctionService for single-label, LabeledLossFunctionService
	// for multi-label).
	singleLabelConfigUncached := lib.ReputerConfig{
		LossFunctionParameters: lib.LossFunctionParameters{
			LossMethodOptions:   reputerOptions,
			LossFunctionService: "scalar-loss-svc",
		},
	}
	multiLabelConfigUncached := lib.ReputerConfig{
		LossFunctionParameters: lib.LossFunctionParameters{
			LossMethodOptions:          reputerOptions,
			LossFunctionService:        "scalar-loss-svc",
			LabeledLossFunctionService: "labeled-loss-svc",
		},
	}

	// Helper: build a LabeledValue slice from (name, decimal-string) pairs.
	labeled := func(pairs ...[2]string) []*emissionstypes.LabeledValue {
		out := make([]*emissionstypes.LabeledValue, 0, len(pairs))
		for i, p := range pairs {
			dec, err := alloraMath.NewDecFromString(p[1])
			require.NoError(t, err)
			out = append(out, &emissionstypes.LabeledValue{
				LabelId:   uint32(i + 1), //nolint:gosec // loop index is small and non-negative
				LabelName: p[0],
				Value:     dec,
			})
		}
		return out
	}

	// Helper: build the labeled prediction slice that computeLoss passes to
	// LabeledLossFunction. Labels are label_0, label_1, ... matching the order in
	// which the value bundles are built above.
	lv := func(vals ...string) []lib.LabeledValue {
		out := make([]lib.LabeledValue, len(vals))
		for i, v := range vals {
			out[i] = lib.LabeledValue{Label: fmt.Sprintf("label_%d", i), Value: v}
		}
		return out
	}

	tests := []struct {
		name          string
		multiLabel    bool
		sourceTruth   []lib.Truth
		valueBundle   *emissionstypes.NetworkInferenceBundle
		reputerConfig lib.ReputerConfig
		mockSetup     func(*MockAlloraAdapter)
		expectError   bool
		errorContains string
		// assertResult runs on the produced bundle for non-error cases. Kept as
		// a per-case closure because the multi-label and one-out/one-in cases
		// assert very different shapes.
		assertResult func(*testing.T, emissionstypes.InputValueBundle)
	}{
		{
			name:        "single label - happy path - all positive values",
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled([2]string{"y", "9.5"}),
				NaiveValue:    labeled([2]string{"y", "9.0"}),
				InfererValues: []*emissionstypes.WorkerInference{
					{Worker: "inferer", Values: labeled([2]string{"y", "9.7"})},
				},
				ForecasterValues: []*emissionstypes.WorkerInference{
					{Worker: "forecaster", Values: labeled([2]string{"y", "9.8"})},
				},
			},
			reputerConfig: singleLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				truth := lib.Truth{Label: "y", Value: "10.0"}
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.5", reputerOptions).Return("0.25", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.0", reputerOptions).Return("1.00", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.7", reputerOptions).Return("0.09", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.8", reputerOptions).Return("0.04", nil)
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				assert.Equal(t, "0.25", result.CombinedValue.String())
				assert.Equal(t, "1.00", result.NaiveValue.String())
				require.Len(t, result.InfererValues, 1)
				assert.Equal(t, "0.09", result.InfererValues[0].Value.String())
				require.Len(t, result.ForecasterValues, 1)
				assert.Equal(t, "0.04", result.ForecasterValues[0].Value.String())
			},
		},
		{
			name:       "multi label - happy path - labeled loss over 3 classes",
			multiLabel: true,
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
				{Label: "label_2", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled(
					[2]string{"label_0", "0.7"},
					[2]string{"label_1", "0.2"},
					[2]string{"label_2", "0.1"},
				),
				NaiveValue: labeled(
					[2]string{"label_0", "0.6"},
					[2]string{"label_1", "0.3"},
					[2]string{"label_2", "0.1"},
				),
				InfererValues: []*emissionstypes.WorkerInference{
					{Worker: "inferer", Values: labeled(
						[2]string{"label_0", "0.8"},
						[2]string{"label_1", "0.1"},
						[2]string{"label_2", "0.1"},
					)},
				},
				ForecasterValues: []*emissionstypes.WorkerInference{
					{Worker: "forecaster", Values: labeled(
						[2]string{"label_0", "0.5"},
						[2]string{"label_1", "0.4"},
						[2]string{"label_2", "0.1"},
					)},
				},
			},
			reputerConfig: multiLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				// The labeled loss function receives the full truth slice and the
				// labeled prediction slice (label-carrying); one call per field.
				m.On("LabeledLossFunction",
					mock.AnythingOfType("lib.ReputerConfig"),
					mock.AnythingOfType("[]lib.Truth"),
					lv("0.7", "0.2", "0.1"), reputerOptions).Return("0.30", nil)
				m.On("LabeledLossFunction",
					mock.AnythingOfType("lib.ReputerConfig"),
					mock.AnythingOfType("[]lib.Truth"),
					lv("0.6", "0.3", "0.1"), reputerOptions).Return("0.40", nil)
				m.On("LabeledLossFunction",
					mock.AnythingOfType("lib.ReputerConfig"),
					mock.AnythingOfType("[]lib.Truth"),
					lv("0.8", "0.1", "0.1"), reputerOptions).Return("0.20", nil)
				m.On("LabeledLossFunction",
					mock.AnythingOfType("lib.ReputerConfig"),
					mock.AnythingOfType("[]lib.Truth"),
					lv("0.5", "0.4", "0.1"), reputerOptions).Return("0.50", nil)
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				// Each field collapses its multi-label vector to one scalar loss.
				assert.Equal(t, "0.30", result.CombinedValue.String())
				assert.Equal(t, "0.40", result.NaiveValue.String())
				require.Len(t, result.InfererValues, 1)
				assert.Equal(t, "0.20", result.InfererValues[0].Value.String())
				require.Len(t, result.ForecasterValues, 1)
				assert.Equal(t, "0.50", result.ForecasterValues[0].Value.String())
			},
		},
		{
			name:       "multi label - one-out and one-in fields with forecaster regroup",
			multiLabel: true,
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				// Combined/Naive present so the bundle is non-empty; their loss
				// values are not the focus of this case.
				CombinedValue: labeled([2]string{"label_0", "0.6"}, [2]string{"label_1", "0.4"}),
				NaiveValue:    labeled([2]string{"label_0", "0.5"}, [2]string{"label_1", "0.5"}),
				OneOutInfererValues: []*emissionstypes.OneOutInfererValue{
					{WithheldInferer: "inf_a", CombinedInference: labeled(
						[2]string{"label_0", "0.7"}, [2]string{"label_1", "0.3"})},
					{WithheldInferer: "inf_b", CombinedInference: labeled(
						[2]string{"label_0", "0.8"}, [2]string{"label_1", "0.2"})},
				},
				OneOutForecasterValues: []*emissionstypes.OneOutForecasterValue{
					{WithheldForecaster: "fc_a", CombinedInference: labeled(
						[2]string{"label_0", "0.55"}, [2]string{"label_1", "0.45"})},
				},
				OneInForecasterValues: []*emissionstypes.OneInForecasterValue{
					{Forecaster: "fc_a", CombinedInference: labeled(
						[2]string{"label_0", "0.65"}, [2]string{"label_1", "0.35"})},
				},
				// Flat: 2 forecasters x 2 withheld inferers = 4 entries that
				// must regroup into 2 nested entries (one per forecaster).
				OneOutInfererForecasterValues: []*emissionstypes.OneOutInfererForecasterValue{
					{Forecaster: "fc_a", WithheldInferer: "inf_a", CombinedInference: labeled(
						[2]string{"label_0", "0.11"}, [2]string{"label_1", "0.89"})},
					{Forecaster: "fc_a", WithheldInferer: "inf_b", CombinedInference: labeled(
						[2]string{"label_0", "0.22"}, [2]string{"label_1", "0.78"})},
					{Forecaster: "fc_b", WithheldInferer: "inf_a", CombinedInference: labeled(
						[2]string{"label_0", "0.33"}, [2]string{"label_1", "0.67"})},
					{Forecaster: "fc_b", WithheldInferer: "inf_b", CombinedInference: labeled(
						[2]string{"label_0", "0.44"}, [2]string{"label_1", "0.56"})},
				},
			},
			reputerConfig: multiLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				labeledLoss := func(values []lib.LabeledValue, ret string) {
					m.On("LabeledLossFunction",
						mock.AnythingOfType("lib.ReputerConfig"),
						mock.AnythingOfType("[]lib.Truth"),
						values, reputerOptions).Return(ret, nil)
				}
				labeledLoss(lv("0.6", "0.4"), "0.10")   // combined
				labeledLoss(lv("0.5", "0.5"), "0.10")   // naive
				labeledLoss(lv("0.7", "0.3"), "0.71")   // one-out inferer inf_a
				labeledLoss(lv("0.8", "0.2"), "0.72")   // one-out inferer inf_b
				labeledLoss(lv("0.55", "0.45"), "0.73") // one-out forecaster fc_a
				labeledLoss(lv("0.65", "0.35"), "0.74") // one-in forecaster fc_a
				labeledLoss(lv("0.11", "0.89"), "0.81") // OOIF fc_a/inf_a
				labeledLoss(lv("0.22", "0.78"), "0.82") // OOIF fc_a/inf_b
				labeledLoss(lv("0.33", "0.67"), "0.83") // OOIF fc_b/inf_a
				labeledLoss(lv("0.44", "0.56"), "0.84") // OOIF fc_b/inf_b
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				// One-out inferer values: flat, scalar loss per withheld inferer.
				require.Len(t, result.OneOutInfererValues, 2)
				assert.Equal(t, "inf_a", result.OneOutInfererValues[0].Worker)
				assert.Equal(t, "0.71", result.OneOutInfererValues[0].Value.String())
				assert.Equal(t, "inf_b", result.OneOutInfererValues[1].Worker)
				assert.Equal(t, "0.72", result.OneOutInfererValues[1].Value.String())

				// One-out forecaster values.
				require.Len(t, result.OneOutForecasterValues, 1)
				assert.Equal(t, "fc_a", result.OneOutForecasterValues[0].Worker)
				assert.Equal(t, "0.73", result.OneOutForecasterValues[0].Value.String())

				// One-in forecaster values.
				require.Len(t, result.OneInForecasterValues, 1)
				assert.Equal(t, "fc_a", result.OneInForecasterValues[0].Worker)
				assert.Equal(t, "0.74", result.OneInForecasterValues[0].Value.String())

				// One-out inferer-forecaster: 4 flat inputs must regroup into 2
				// nested entries (fc_a, fc_b), first-seen order, each with 2
				// per-inferer values.
				require.Len(t, result.OneOutInfererForecasterValues, 2)

				fcA := result.OneOutInfererForecasterValues[0]
				assert.Equal(t, "fc_a", fcA.Forecaster)
				require.Len(t, fcA.OneOutInfererValues, 2)
				assert.Equal(t, "inf_a", fcA.OneOutInfererValues[0].Worker)
				assert.Equal(t, "0.81", fcA.OneOutInfererValues[0].Value.String())
				assert.Equal(t, "inf_b", fcA.OneOutInfererValues[1].Worker)
				assert.Equal(t, "0.82", fcA.OneOutInfererValues[1].Value.String())

				fcB := result.OneOutInfererForecasterValues[1]
				assert.Equal(t, "fc_b", fcB.Forecaster)
				require.Len(t, fcB.OneOutInfererValues, 2)
				assert.Equal(t, "inf_a", fcB.OneOutInfererValues[0].Worker)
				assert.Equal(t, "0.83", fcB.OneOutInfererValues[0].Value.String())
				assert.Equal(t, "inf_b", fcB.OneOutInfererValues[1].Worker)
				assert.Equal(t, "0.84", fcB.OneOutInfererValues[1].Value.String())
			},
		},
		{
			name:       "multi label values but no LabeledLossFunctionService configured",
			multiLabel: true,
			// A multi-label topic whose config lacks a labeled loss service is an
			// explicit error rather than silently falling back to scalar loss.
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled(
					[2]string{"label_0", "0.7"},
					[2]string{"label_1", "0.3"},
				),
			},
			reputerConfig: singleLabelConfig, // LabeledLossFunctionService == ""
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "requires a LabeledLossFunctionService",
		},
		{
			name:        "error in LossFunction",
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled([2]string{"y", "9.5"}),
			},
			reputerConfig: singleLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("loss function error"))
			},
			expectError:   true,
			errorContains: "error computing loss for combined value",
		},
		{
			name:       "error in LabeledLossFunction",
			multiLabel: true,
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled([2]string{"label_0", "0.7"}, [2]string{"label_1", "0.3"}),
			},
			reputerConfig: multiLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LabeledLossFunction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("labeled loss error"))
			},
			expectError:   true,
			errorContains: "error computing loss for combined value",
		},
		{
			name:        "invalid loss value",
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled([2]string{"y", "9.5"}),
			},
			reputerConfig: singleLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("invalid", nil)
			},
			expectError:   true,
			errorContains: "error parsing loss",
		},
		{
			name:          "nil ValueBundle",
			sourceTruth:   []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle:   nil,
			reputerConfig: singleLabelConfig,
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "nil ValueBundle",
		},
		{
			name:          "empty ValueBundle",
			sourceTruth:   []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle:   &emissionstypes.NetworkInferenceBundle{},
			reputerConfig: singleLabelConfig,
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "empty ValueBundle",
		},
		{
			name:       "multi label values (3 classes) but no LabeledLossFunctionService configured",
			multiLabel: true,
			// 3-class multi-label topic whose config lacks a labeled loss service
			// must error rather than fall back to scalar loss.
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
				{Label: "label_2", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled(
					[2]string{"label_0", "0.7"},
					[2]string{"label_1", "0.2"},
					[2]string{"label_2", "0.1"},
				),
			},
			reputerConfig: singleLabelConfig, // LabeledLossFunctionService == ""
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "requires a LabeledLossFunctionService",
		},
		{
			name: "single-label topic with a stray LabeledLossFunctionService - uses scalar loss",
			// A SINGLE-arity topic always uses the scalar LossFunction, even when a
			// LabeledLossFunctionService is also configured (the stray endpoint is
			// ignored). multiLabel defaults to false.
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{ //nolint:exhaustruct
				CombinedValue: labeled([2]string{"y", "9.5"}),
				NaiveValue:    labeled([2]string{"y", "9.0"}),
			},
			reputerConfig: bothServicesConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				truth := lib.Truth{Label: "y", Value: "10.0"}
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.5", reputerOptions).Return("0.25", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.0", reputerOptions).Return("1.00", nil)
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				assert.Equal(t, "0.25", result.CombinedValue.String())
				assert.Equal(t, "1.00", result.NaiveValue.String())
			},
		},
		{
			name: "single label value with single-label config - still works",
			// Regression guard: the ordinary single-label path must keep working
			// after the stricter branching.
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{
				CombinedValue: labeled([2]string{"y", "9.5"}),
				NaiveValue:    labeled([2]string{"y", "9.0"}),
			},
			reputerConfig: singleLabelConfig,
			mockSetup: func(m *MockAlloraAdapter) {
				truth := lib.Truth{Label: "y", Value: "10.0"}
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.5", reputerOptions).Return("0.25", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.0", reputerOptions).Return("1.00", nil)
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				assert.Equal(t, "0.25", result.CombinedValue.String())
				assert.Equal(t, "1.00", result.NaiveValue.String())
			},
		},
		{
			name: "single label - never-negative resolved against scalar service and cached",
			// IsNeverNegative is not cached, so it must be queried once against the
			// scalar LossFunctionService and reused for the remaining fields.
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{ //nolint:exhaustruct
				CombinedValue: labeled([2]string{"y", "9.5"}),
				NaiveValue:    labeled([2]string{"y", "9.0"}),
			},
			reputerConfig: singleLabelConfigUncached,
			mockSetup: func(m *MockAlloraAdapter) {
				truth := lib.Truth{Label: "y", Value: "10.0"}
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.5", reputerOptions).Return("0.25", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.0", reputerOptions).Return("1.00", nil)
				// .Once() asserts the result is cached: a single query covers both fields.
				m.On("IsLossFunctionNeverNegative", mock.AnythingOfType("lib.ReputerConfig"), reputerOptions, "scalar-loss-svc").Return(false, nil).Once()
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				assert.Equal(t, "0.25", result.CombinedValue.String())
				assert.Equal(t, "1.00", result.NaiveValue.String())
			},
		},
		{
			name:       "multi label - never-negative resolved against labeled service",
			multiLabel: true,
			// Multi-label topics must query the LabeledLossFunctionService, not the
			// scalar one, for the never-negative check.
			sourceTruth: []lib.Truth{
				{Label: "label_0", Value: "1.0"},
				{Label: "label_1", Value: "0.0"},
			},
			valueBundle: &emissionstypes.NetworkInferenceBundle{ //nolint:exhaustruct
				CombinedValue: labeled([2]string{"label_0", "0.7"}, [2]string{"label_1", "0.3"}),
				NaiveValue:    labeled([2]string{"label_0", "0.6"}, [2]string{"label_1", "0.4"}),
			},
			reputerConfig: multiLabelConfigUncached,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LabeledLossFunction", mock.AnythingOfType("lib.ReputerConfig"), mock.AnythingOfType("[]lib.Truth"), lv("0.7", "0.3"), reputerOptions).Return("0.30", nil)
				m.On("LabeledLossFunction", mock.AnythingOfType("lib.ReputerConfig"), mock.AnythingOfType("[]lib.Truth"), lv("0.6", "0.4"), reputerOptions).Return("0.40", nil)
				m.On("IsLossFunctionNeverNegative", mock.AnythingOfType("lib.ReputerConfig"), reputerOptions, "labeled-loss-svc").Return(false, nil).Once()
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				assert.Equal(t, "0.30", result.CombinedValue.String())
				assert.Equal(t, "0.40", result.NaiveValue.String())
			},
		},
		{
			name:        "single label - never-negative true applies Log10 transform",
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{ //nolint:exhaustruct
				CombinedValue: labeled([2]string{"y", "9.5"}),
				NaiveValue:    labeled([2]string{"y", "9.0"}),
			},
			reputerConfig: singleLabelConfigUncached,
			mockSetup: func(m *MockAlloraAdapter) {
				truth := lib.Truth{Label: "y", Value: "10.0"}
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.5", reputerOptions).Return("10.0", nil)
				m.On("LossFunction", mock.AnythingOfType("lib.ReputerConfig"), truth, "9.0", reputerOptions).Return("100.0", nil)
				m.On("IsLossFunctionNeverNegative", mock.AnythingOfType("lib.ReputerConfig"), reputerOptions, "scalar-loss-svc").Return(true, nil)
			},
			assertResult: func(t *testing.T, result emissionstypes.InputValueBundle) {
				t.Helper()
				// When the loss function is never negative, the loss is Log10-transformed
				// before being stored. Mirror the production transform to stay robust to
				// the exact decimal formatting.
				combinedLog, err := alloraMath.Log10(alloraMath.MustNewDecFromString("10.0"))
				require.NoError(t, err)
				combinedBounded, err := alloraMath.NewBoundedExp40Dec(combinedLog)
				require.NoError(t, err)
				assert.Equal(t, combinedBounded.String(), result.CombinedValue.String())

				naiveLog, err := alloraMath.Log10(alloraMath.MustNewDecFromString("100.0"))
				require.NoError(t, err)
				naiveBounded, err := alloraMath.NewBoundedExp40Dec(naiveLog)
				require.NoError(t, err)
				assert.Equal(t, naiveBounded.String(), result.NaiveValue.String())
			},
		},
		{
			name:        "error from never-negative check propagates",
			sourceTruth: []lib.Truth{{Label: "y", Value: "10.0"}},
			valueBundle: &emissionstypes.NetworkInferenceBundle{ //nolint:exhaustruct
				CombinedValue: labeled([2]string{"y", "9.5"}),
			},
			reputerConfig: singleLabelConfigUncached,
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LossFunction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("0.25", nil)
				m.On("IsLossFunctionNeverNegative", mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("never-negative service unreachable"))
			},
			expectError:   true,
			errorContains: "failed to determine if loss function is never negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := ReturnBasicMockAlloraAdapter()
			tt.mockSetup(mockAdapter)
			tt.reputerConfig.GroundTruthEntrypoint = mockAdapter
			tt.reputerConfig.LossFunctionEntrypoint = mockAdapter

			mockConnectionManager := &lib.MockConnectionManager{}
			mockConnectionManager.On("GetWallet").Return(&lib.Wallet{Address: "address", AddressSDK: nil}, nil)

			suite := &UseCaseSuite{
				ConnectionManager: mockConnectionManager,
			}
			result, err := suite.ComputeLossBundle(tt.sourceTruth, tt.valueBundle, tt.reputerConfig, tt.multiLabel)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.assertResult != nil {
					tt.assertResult(t, result)
				}
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}

// TestGetSourceTruthDispatch verifies that getSourceTruth dispatches to the correct
// ground-truth entrypoint method depending on whether the reputer is configured for
// multi-label ground truth (LabeledGroundTruthEndpoint present) or scalar ground
// truth. The mock fails the test if an unexpected method is called, so each case
// implicitly asserts the other path is NOT taken.
//
//nolint:exhaustruct
func TestGetSourceTruthDispatch(t *testing.T) {
	const (
		address = "reputer1"
		nonce   = lib.BlockHeight(10)
	)

	tests := []struct {
		name          string
		multiLabel    bool
		parameters    map[string]string
		mockSetup     func(*MockAlloraAdapter)
		expectError   bool
		errorContains string
		assertResult  func(*testing.T, []lib.Truth)
	}{
		{
			name:       "scalar topic dispatches to GroundTruth",
			multiLabel: false,
			parameters: map[string]string{lib.ParamGroundTruthEndpoint: "http://x/gt"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("GroundTruth", mock.AnythingOfType("lib.ReputerConfig"), nonce).
					Return(lib.Truth{Value: "0.5"}, nil).Once()
			},
			assertResult: func(t *testing.T, truths []lib.Truth) {
				t.Helper()
				// Scalar truth is wrapped into a single-element slice.
				require.Len(t, truths, 1)
				assert.Equal(t, "0.5", truths[0].Value)
			},
		},
		{
			name:       "multi-label topic dispatches to LabeledGroundTruth",
			multiLabel: true,
			parameters: map[string]string{lib.ParamLabeledGroundTruthEndpoint: "http://x/labeled-gt"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LabeledGroundTruth", mock.AnythingOfType("lib.ReputerConfig"), nonce).
					Return([]lib.Truth{{Label: "UP", Value: "1.0"}, {Label: "DOWN", Value: "0.0"}}, nil).Once()
			},
			assertResult: func(t *testing.T, truths []lib.Truth) {
				t.Helper()
				require.Len(t, truths, 2)
				assert.Equal(t, "UP", truths[0].Label)
				assert.Equal(t, "1.0", truths[0].Value)
				assert.Equal(t, "DOWN", truths[1].Label)
				assert.Equal(t, "0.0", truths[1].Value)
			},
		},
		{
			// Arity wins over a contradicting endpoint.
			name:       "scalar topic ignores stray LabeledGroundTruthEndpoint",
			multiLabel: false,
			parameters: map[string]string{
				lib.ParamGroundTruthEndpoint:        "http://x/gt",
				lib.ParamLabeledGroundTruthEndpoint: "http://x/labeled-gt",
			},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("GroundTruth", mock.AnythingOfType("lib.ReputerConfig"), int64(nonce)).
					Return(lib.Truth{Value: "0.5"}, nil).Once()
			},
			assertResult: func(t *testing.T, truths []lib.Truth) {
				require.Len(t, truths, 1)
				assert.Equal(t, "0.5", truths[0].Value)
			},
		},
		{
			name:       "multi-label topic ignores stray GroundTruthEndpoint",
			multiLabel: true,
			parameters: map[string]string{
				lib.ParamGroundTruthEndpoint:        "http://x/gt",
				lib.ParamLabeledGroundTruthEndpoint: "http://x/labeled-gt",
			},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LabeledGroundTruth", mock.AnythingOfType("lib.ReputerConfig"), int64(nonce)).
					Return([]lib.Truth{{Label: "UP", Value: "1.0"}}, nil).Once()
			},
			assertResult: func(t *testing.T, truths []lib.Truth) {
				require.Len(t, truths, 1)
			},
		},
		{
			name:          "multi-label topic without LabeledGroundTruthEndpoint errors",
			multiLabel:    true,
			parameters:    map[string]string{lib.ParamGroundTruthEndpoint: "http://x/gt"},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "no LabeledGroundTruthEndpoint is configured",
		},
		{
			name:          "scalar topic without GroundTruthEndpoint errors",
			multiLabel:    false,
			parameters:    map[string]string{lib.ParamLabeledGroundTruthEndpoint: "http://x/labeled-gt"},
			mockSetup:     func(m *MockAlloraAdapter) {},
			expectError:   true,
			errorContains: "no GroundTruthEndpoint is configured",
		},
		{
			name:       "scalar ground truth error propagates",
			multiLabel: false,
			parameters: map[string]string{lib.ParamGroundTruthEndpoint: "http://x/gt"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("GroundTruth", mock.AnythingOfType("lib.ReputerConfig"), nonce).
					Return(lib.Truth{}, errors.New("gt endpoint down")).Once()
			},
			expectError:   true,
			errorContains: "error getting source truth from reputer",
		},
		{
			name:       "labeled ground truth error propagates",
			multiLabel: true,
			parameters: map[string]string{lib.ParamLabeledGroundTruthEndpoint: "http://x/labeled-gt"},
			mockSetup: func(m *MockAlloraAdapter) {
				m.On("LabeledGroundTruth", mock.AnythingOfType("lib.ReputerConfig"), nonce).
					Return([]lib.Truth(nil), errors.New("labeled gt endpoint down")).Once()
			},
			expectError:   true,
			errorContains: "error getting labeled source truth from reputer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter := ReturnBasicMockAlloraAdapter()
			tt.mockSetup(mockAdapter)

			reputer := lib.ReputerConfig{
				TopicId:               emissionstypes.TopicId(1),
				GroundTruthEntrypoint: mockAdapter,
				GroundTruthParameters: tt.parameters,
			}

			suite := &UseCaseSuite{Metrics: &metrics.Metrics{}}

			truths, err := suite.getSourceTruth(reputer, tt.multiLabel, nonce, address)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.assertResult != nil {
					tt.assertResult(t, truths)
				}
			}

			mockAdapter.AssertExpectations(t)
		})
	}
}
