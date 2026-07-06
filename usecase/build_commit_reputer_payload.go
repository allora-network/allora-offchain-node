package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"allora_offchain_node/lib"
	"allora_offchain_node/lib/auth"
	"allora_offchain_node/metrics"

	errorsmod "cosmossdk.io/errors"
	"github.com/rs/zerolog/log"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

// Get the reputer's values at the block from the chain
// Compute loss bundle with the reputer provided Loss function and ground truth
// sign and commit to chain
// multiLabel is resolved once at startup from the topic's immutable on-chain
// OutputArity (see startReputer) and threaded in, rather than re-queried per
// submission.
func (suite *UseCaseSuite) BuildCommitReputerPayload(ctx context.Context, reputer lib.ReputerConfig, nonce lib.BlockHeight, timeoutHeight uint64, multiLabel bool) error {
	log := log.With().Uint64("topicId", reputer.TopicId).Str("actorType", "reputer").Logger()
	log.Info().Msg("Building reputer payload")
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet")
	}
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet config")
	}

	networkInferenceBundle, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (*emissionstypes.NetworkInferenceBundle, error) {
			return node.GetReputerValuesAtBlock(ctx, reputer.TopicId, nonce)
		},
		"get reputer values",
		lib.GRPC_MODE,
	)
	if err != nil {
		return errorsmod.Wrapf(err, "error getting reputer values, topic: %d, blockHeight: %d", reputer.TopicId, nonce)
	}
	networkInferenceBundle.Nonce = nonce

	sourceTruth, err := suite.getSourceTruth(reputer, multiLabel, nonce, wallet.Address)
	if err != nil {
		return err
	}

	lossBundle, err := suite.ComputeLossBundle(sourceTruth, networkInferenceBundle, reputer, multiLabel)
	if err != nil {
		return errorsmod.Wrapf(err, "error computing loss bundle, topic: %d, blockHeight: %d", reputer.TopicId, nonce)
	}

	signedValueBundle, err := suite.SignReputerValueBundle(&lossBundle)
	if err != nil {
		return errorsmod.Wrapf(err, "error signing reputer value bundle, topic: %d, blockHeight: %d", reputer.TopicId, nonce)
	}

	if err := signedValueBundle.Validate(); err != nil {
		return errorsmod.Wrapf(err, "error validating reputer value bundle, topic: %d, blockHeight: %d", reputer.TopicId, nonce)
	}

	req := &emissionstypes.InsertReputerPayloadRequest{
		Sender:             wallet.Address,
		ReputerValueBundle: signedValueBundle,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Msgf("Error marshaling MsgInserReputerPayload to print Msg as JSON")
	} else {
		log.Debug().Msgf("Sending InsertReputerPayload to chain %s", string(reqJSON))
	}

	if walletConfig.SubmitTx {
		_, err = suite.ConnectionManager.SendDataWithNodeRetry(ctx, req, timeoutHeight, "Send Reputer Data to chain")
		if err != nil {
			return errorsmod.Wrapf(err, "error sending Reputer Data to chain, topic: %d, blockHeight: %d", reputer.TopicId, nonce)
		}
		suite.Metrics.IncrementMetricsCounter(metrics.ReputerChainSubmissionCount, wallet.Address, reputer.TopicId)
	} else {
		log.Info().Msg("SubmitTx=false; Skipping sending Reputer Data to chain")
	}

	return nil
}

// getSourceTruth fetches the reputer's source of truth from the configured
// entrypoint. It dispatches on the topic's on-chain output arity (multiLabel): a
// multi-label topic fetches the labeled ground truth, a single-label topic the
// scalar one (returned as a single-element slice). If the endpoint required for
// the topic's arity is missing it errors; if a contradicting endpoint is also
// present it warns and ignores it.
func (suite *UseCaseSuite) getSourceTruth(reputer lib.ReputerConfig, multiLabel bool, nonce lib.BlockHeight, walletAddress string) ([]lib.Truth, error) {
	log := log.With().Uint64("topicId", reputer.TopicId).Str("actorType", "reputer").Logger()
	_, hasLabeled := reputer.GroundTruthParameters[lib.ParamLabeledGroundTruthEndpoint]
	_, hasScalar := reputer.GroundTruthParameters[lib.ParamGroundTruthEndpoint]

	if multiLabel {
		if !hasLabeled {
			return nil, errorsmod.Wrapf(emissionstypes.ErrInvalidValue,
				"topic %d is multi-label (MULTI) but no %s is configured", reputer.TopicId, lib.ParamLabeledGroundTruthEndpoint)
		}
		if hasScalar {
			log.Warn().Msgf("topic is multi-label but %s is also configured; ignoring it", lib.ParamGroundTruthEndpoint)
		}
		sourceTruth, err := reputer.GroundTruthEntrypoint.LabeledGroundTruth(reputer, nonce)
		if err != nil {
			return nil, errorsmod.Wrapf(err, "error getting labeled source truth from reputer, topicId: %d, blockHeight: %d", reputer.TopicId, nonce)
		}
		suite.Metrics.IncrementMetricsCounter(metrics.TruthRequestCount, walletAddress, reputer.TopicId)
		return sourceTruth, nil
	}

	if !hasScalar {
		return nil, errorsmod.Wrapf(emissionstypes.ErrInvalidValue,
			"topic %d is single-label (SINGLE) but no %s is configured", reputer.TopicId, lib.ParamGroundTruthEndpoint)
	}
	if hasLabeled {
		log.Warn().Msgf("topic is single-label but %s is also configured; ignoring it", lib.ParamLabeledGroundTruthEndpoint)
	}
	truth, err := reputer.GroundTruthEntrypoint.GroundTruth(reputer, nonce)
	if err != nil {
		return nil, errorsmod.Wrapf(err, "error getting source truth from reputer, topicId: %d, blockHeight: %d", reputer.TopicId, nonce)
	}
	suite.Metrics.IncrementMetricsCounter(metrics.TruthRequestCount, walletAddress, reputer.TopicId)
	return []lib.Truth{truth}, nil
}

// buildLabeledPredictions converts the predicted labeled values into a
// []lib.LabeledValue and verifies that the prediction labels match the ground
// truth labels exactly (same set, no missing/extra/duplicate labels on either
// side). Carrying labels lets the loss service join predictions to ground truth
// by label rather than by array position, and the strict validation fails loudly
// on a label mismatch instead of silently computing a wrong loss.
func buildLabeledPredictions(values []*emissionstypes.LabeledValue, sourceTruth []lib.Truth) ([]lib.LabeledValue, error) {
	truthLabels := make(map[string]struct{}, len(sourceTruth))
	for _, t := range sourceTruth {
		if _, dup := truthLabels[t.Label]; dup {
			return nil, fmt.Errorf("duplicate label %q in ground truth", t.Label)
		}
		truthLabels[t.Label] = struct{}{}
	}

	predictions := make([]lib.LabeledValue, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == nil {
			return nil, errors.New("nil predicted value")
		}
		if _, dup := seen[v.LabelName]; dup {
			return nil, fmt.Errorf("duplicate label %q in predicted values", v.LabelName)
		}
		seen[v.LabelName] = struct{}{}
		if _, ok := truthLabels[v.LabelName]; !ok {
			return nil, fmt.Errorf("predicted label %q has no matching ground truth label", v.LabelName)
		}
		predictions = append(predictions, lib.LabeledValue{Label: v.LabelName, Value: v.Value.String()})
	}

	if len(predictions) != len(sourceTruth) {
		return nil, fmt.Errorf(
			"label count mismatch: %d predicted labels vs %d ground truth labels",
			len(predictions), len(sourceTruth))
	}

	return predictions, nil
}

func (suite *UseCaseSuite) ComputeLossBundle(sourceTruth []lib.Truth, vb *emissionstypes.NetworkInferenceBundle, reputer lib.ReputerConfig, multiLabel bool) (emissionstypes.InputValueBundle, error) {
	if vb == nil {
		return emissionstypes.InputValueBundle{}, errors.New("nil ValueBundle")
	}
	// Check if vb is empty
	if IsEmpty(*vb) {
		return emissionstypes.InputValueBundle{}, errors.New("empty ValueBundle")
	}

	combinedValues := emissionstypes.ConvertLabeledValuesToDecArray(vb.CombinedValue)
	if err := emissionstypes.ValidateDecs(combinedValues); err != nil {
		return emissionstypes.InputValueBundle{}, errors.New("ValueBundle - invalid CombinedValue")
	}
	naiveValues := emissionstypes.ConvertLabeledValuesToDecArray(vb.NaiveValue)
	if err := emissionstypes.ValidateDecs(naiveValues); err != nil {
		return emissionstypes.InputValueBundle{}, errors.New("ValueBundle - invalid NaiveValue")
	}

	lossMethodOptions := reputer.LossFunctionParameters.LossMethodOptions

	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "failed to get wallet")
	}
	losses := emissionstypes.InputValueBundle{ //nolint:exhaustruct
		TopicId: vb.TopicId,
		ReputerRequestNonce: &emissionstypes.ReputerRequestNonce{
			ReputerNonce: &emissionstypes.Nonce{
				BlockHeight: vb.Nonce,
			},
		},
		Reputer: wallet.Address,
	}

	// resolveIsNeverNegative determines, and caches on the reputer config, whether
	// the loss function served at the given endpoint is never negative. The scalar
	// and labeled loss services may differ, so the right endpoint is only known
	// once we know whether a given value vector is single- or multi-label.
	resolveIsNeverNegative := func(serviceEndpoint string) (bool, error) {
		if reputer.LossFunctionParameters.IsNeverNegative != nil {
			return *reputer.LossFunctionParameters.IsNeverNegative, nil
		}
		isNeverNegative, err := reputer.LossFunctionEntrypoint.IsLossFunctionNeverNegative(reputer, lossMethodOptions, serviceEndpoint)
		if err != nil {
			return false, err
		}
		// cache the result
		reputer.LossFunctionParameters.IsNeverNegative = &isNeverNegative
		return isNeverNegative, nil
	}

	computeLoss := func(values []*emissionstypes.LabeledValue, description string) (alloraMath.Dec, error) {
		if len(values) == 0 {
			return alloraMath.Dec{}, errors.New("no values provided to compute loss")
		}

		var lossStr string
		// serviceEndpoint is the loss service used for this value vector; it also
		// determines which endpoint the never-negative check is made against. The
		// scalar-vs-labeled choice is driven by the topic's on-chain arity, not the
		// runtime vector length: a multi-label topic can legitimately produce a
		// length-1 vector at a block, which must still use the labeled loss service.
		var serviceEndpoint string
		if multiLabel {
			// Multi-label: requires the labeled loss service.
			if reputer.LossFunctionParameters.LabeledLossFunctionService == "" {
				return alloraMath.Dec{}, errorsmod.Wrapf(
					emissionstypes.ErrInvalidValue,
					"multi-label topic requires a LabeledLossFunctionService for %s, but none is configured",
					description)
			}
			// Carry labels through to the loss service and verify the predicted
			// labels match the ground-truth labels exactly. This makes the loss a
			// per-label computation instead of relying on the array order of the
			// value bundle and the ground-truth source, which can diverge.
			labeledPredictions, err := buildLabeledPredictions(values, sourceTruth)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error aligning labeled values for %s", description)
			}
			serviceEndpoint = reputer.LossFunctionParameters.LabeledLossFunctionService
			ls, err := reputer.LossFunctionEntrypoint.LabeledLossFunction(
				reputer, sourceTruth, labeledPredictions, lossMethodOptions)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error computing labeled loss for %s", description)
			}
			lossStr = ls
		} else {
			// Single-label (scalar): requires the scalar loss service and exactly
			// one value/truth.
			if reputer.LossFunctionParameters.LossFunctionService == "" {
				return alloraMath.Dec{}, errorsmod.Wrapf(
					emissionstypes.ErrInvalidValue,
					"single-label topic requires a LossFunctionService for %s, but none is configured",
					description)
			}
			if len(sourceTruth) == 0 {
				return alloraMath.Dec{}, errorsmod.Wrapf(
					emissionstypes.ErrInvalidValue,
					"single-label value for %s but no source truth provided", description)
			}
			if len(values) != 1 {
				return alloraMath.Dec{}, errorsmod.Wrapf(
					emissionstypes.ErrInvalidValue,
					"single-label topic expects exactly one value for %s, got %d", description, len(values))
			}
			serviceEndpoint = reputer.LossFunctionParameters.LossFunctionService
			ls, err := reputer.LossFunctionEntrypoint.LossFunction(
				reputer, sourceTruth[0], values[0].Value.String(), lossMethodOptions)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error computing loss for %s", description)
			}
			lossStr = ls
		}

		loss, err := alloraMath.NewDecFromString(lossStr)
		if err != nil {
			return alloraMath.Dec{}, errorsmod.Wrapf(err, "error parsing loss value for %s", description)
		}

		isNeverNegative, err := resolveIsNeverNegative(serviceEndpoint)
		if err != nil {
			return alloraMath.Dec{}, errorsmod.Wrapf(err, "failed to determine if loss function is never negative for %s", description)
		}
		if isNeverNegative {
			loss, err = alloraMath.Log10(loss)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error Log10 for %s", description)
			}
		}

		if err := emissionstypes.ValidateDec(loss); err != nil {
			return alloraMath.Dec{}, errorsmod.Wrapf(err, "invalid loss value for %s", description)
		}

		return loss, nil
	}

	// Combined Value
	if combinedLoss, err := computeLoss(vb.CombinedValue, "combined value"); err != nil {
		return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for combined value")
	} else {
		losses.CombinedValue, err = alloraMath.NewBoundedExp40Dec(combinedLoss)
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting combined loss to BoundedExp40Dec")
		}
	}

	// Naive Value
	if naiveLoss, err := computeLoss(vb.NaiveValue, "naive value"); err != nil {
		return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for naive value")
	} else {
		losses.NaiveValue, err = alloraMath.NewBoundedExp40Dec(naiveLoss)
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
		}
	}

	// Inferer Values
	losses.InfererValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.InfererValues))
	for i, val := range vb.InfererValues {
		if loss, err := computeLoss(val.Values, fmt.Sprintf("inferer value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for inferer value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.InfererValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Worker, Value: boundedLoss}
		}
	}

	// Forecaster Values
	losses.ForecasterValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.ForecasterValues))
	for i, val := range vb.ForecasterValues {
		if loss, err := computeLoss(val.Values, fmt.Sprintf("forecaster value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for forecaster value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.ForecasterValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Worker, Value: boundedLoss}
		}
	}

	// One Out Inferer Values
	losses.OneOutInfererValues = make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(vb.OneOutInfererValues))
	for i, val := range vb.OneOutInfererValues {
		if loss, err := computeLoss(val.CombinedInference, fmt.Sprintf("one out inferer value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for one-out inferer value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.OneOutInfererValues[i] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: val.WithheldInferer, Value: boundedLoss}
		}
	}

	// One Out Forecaster Values
	losses.OneOutForecasterValues = make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(vb.OneOutForecasterValues))
	for i, val := range vb.OneOutForecasterValues {
		if loss, err := computeLoss(val.CombinedInference, fmt.Sprintf("one out forecaster value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for one-out forecaster value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.OneOutForecasterValues[i] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: val.WithheldForecaster, Value: boundedLoss}
		}
	}

	// One In Forecaster Values
	losses.OneInForecasterValues = make([]*emissionstypes.InputWorkerAttributedValue, len(vb.OneInForecasterValues))
	for i, val := range vb.OneInForecasterValues {
		if loss, err := computeLoss(val.CombinedInference, fmt.Sprintf("one in forecaster value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for one-in forecaster value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.OneInForecasterValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Forecaster, Value: boundedLoss}
		}
	}

	// vb.OneOutInfererForecasterValues is flat: one entry per (forecaster, withheld
	// inferer) pair, each carrying a multi-label CombinedInference vector. The
	// target type is nested: one entry per forecaster, holding per-inferer scalar
	// losses. So we regroup by forecaster, preserving first-seen order.
	groupOrder := make([]string, 0)
	grouped := make(map[string][]*emissionstypes.InputWithheldWorkerAttributedValue)

	for i, val := range vb.OneOutInfererForecasterValues {
		loss, err := computeLoss(val.CombinedInference, fmt.Sprintf("one-out inferer-forecaster value %d", i))
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err,
				"error computing loss for one-out inferer-forecaster value")
		}

		boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err,
				"error converting one-out inferer-forecaster loss to BoundedExp40Dec")
		}

		if _, seen := grouped[val.Forecaster]; !seen {
			groupOrder = append(groupOrder, val.Forecaster)
		}
		grouped[val.Forecaster] = append(grouped[val.Forecaster],
			&emissionstypes.InputWithheldWorkerAttributedValue{
				Worker: val.WithheldInferer,
				Value:  boundedLoss,
			})
	}

	losses.OneOutInfererForecasterValues = make([]*emissionstypes.InputOneOutInfererForecasterValues, 0, len(groupOrder))
	for _, forecaster := range groupOrder {
		losses.OneOutInfererForecasterValues = append(losses.OneOutInfererForecasterValues,
			&emissionstypes.InputOneOutInfererForecasterValues{
				Forecaster:          forecaster,
				OneOutInfererValues: grouped[forecaster],
			})
	}

	return losses, nil
}

func (suite *UseCaseSuite) SignReputerValueBundle(valueBundle *emissionstypes.InputValueBundle) (*emissionstypes.InputReputerValueBundle, error) {
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return &emissionstypes.InputReputerValueBundle{}, errorsmod.Wrapf(err, "error getting wallet") //nolint:exhaustruct
	}
	sig, pk, err := auth.MarshalAndSignByPrivKey(valueBundle, wallet.GetPrivKey(), wallet.AddressSDK)
	if err != nil {
		return &emissionstypes.InputReputerValueBundle{}, errorsmod.Wrapf(err, "error signing the InferenceForecastsBundle message") //nolint:exhaustruct
	}
	pkStr := hex.EncodeToString(pk)
	reputerValueBundle := &emissionstypes.InputReputerValueBundle{
		ValueBundle: valueBundle,
		Signature:   sig,
		Pubkey:      pkStr,
	}

	return reputerValueBundle, nil
}
