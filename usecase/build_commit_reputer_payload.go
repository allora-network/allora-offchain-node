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
func (suite *UseCaseSuite) BuildCommitReputerPayload(ctx context.Context, reputer lib.ReputerConfig, nonce lib.BlockHeight, timeoutHeight uint64) error {
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

	var sourceTruth []lib.Truth
	if _, ok := reputer.GroundTruthParameters["LabeledGroundTruthEndpoint"]; ok {
		sourceTruth, err = reputer.GroundTruthEntrypoint.LabeledGroundTruth(reputer, nonce)
		if err != nil {
			return errorsmod.Wrapf(err, "error getting labeled source truth from reputer, topicId: %d, blockHeight: %d", reputer.TopicId, nonce)
		}
		suite.Metrics.IncrementMetricsCounter(metrics.TruthRequestCount, wallet.Address, reputer.TopicId)
	} else {
		truth, err := reputer.GroundTruthEntrypoint.GroundTruth(reputer, nonce)
		if err != nil {
			return errorsmod.Wrapf(err, "error getting source truth from reputer, topicId: %d, blockHeight: %d", reputer.TopicId, nonce)
		}
		sourceTruth = append(sourceTruth, truth)
		suite.Metrics.IncrementMetricsCounter(metrics.TruthRequestCount, wallet.Address, reputer.TopicId)
	}

	lossBundle, err := suite.ComputeLossBundle(sourceTruth, networkInferenceBundle, reputer)
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

func (suite *UseCaseSuite) ComputeLossBundle(sourceTruth []lib.Truth, vb *emissionstypes.NetworkInferenceBundle, reputer lib.ReputerConfig) (emissionstypes.InputValueBundle, error) {
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
	// Use the cached IsNeverNegative value
	isNeverNegative := false
	if reputer.LossFunctionParameters.IsNeverNegative != nil {
		isNeverNegative = *reputer.LossFunctionParameters.IsNeverNegative
	} else {
		var err error
		isNeverNegative, err = reputer.LossFunctionEntrypoint.IsLossFunctionNeverNegative(reputer, lossMethodOptions)
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "failed to determine if loss function is never negative")
		}
		// cache the result
		reputer.LossFunctionParameters.IsNeverNegative = &isNeverNegative
	}

	wallet, _ := suite.ConnectionManager.GetWallet()
	losses := emissionstypes.InputValueBundle{ //nolint:exhaustruct
		TopicId: vb.TopicId,
		ReputerRequestNonce: &emissionstypes.ReputerRequestNonce{
			ReputerNonce: &emissionstypes.Nonce{
				BlockHeight: vb.Nonce,
			},
		},
		Reputer: wallet.Address,
	}

	computeLoss := func(value alloraMath.DecArray, description string) (alloraMath.Dec, error) {
		lenValue := len(value)
		if lenValue == 0 {
			return alloraMath.Dec{}, errors.New("no values provided to compute loss")
		}
		valuesStr := make([]string, lenValue)
		for i := range value {
			valuesStr[i] = value[i].String()
		}

		var (
			lossStr string
			err     error
		)
		if reputer.LossFunctionParameters.LabeledLossFunctionService != "" {
			lossStr, err = reputer.LossFunctionEntrypoint.LabeledLossFunction(reputer, sourceTruth, valuesStr, lossMethodOptions)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error computing labeled loss for %s", description)
			}
		} else {
			lossStr, err = reputer.LossFunctionEntrypoint.LossFunction(reputer, sourceTruth[0], valuesStr[0], lossMethodOptions)
			if err != nil {
				return alloraMath.Dec{}, errorsmod.Wrapf(err, "error computing loss for %s", description)
			}
		}

		loss, err := alloraMath.NewDecFromString(lossStr)
		if err != nil {
			return alloraMath.Dec{}, errorsmod.Wrapf(err, "error parsing loss value for %s", description)
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
	if combinedLoss, err := computeLoss(combinedValues, "combined value"); err != nil {
		return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for combined value")
	} else {
		losses.CombinedValue, err = alloraMath.NewBoundedExp40Dec(combinedLoss)
		if err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting combined loss to BoundedExp40Dec")
		}
	}

	// Naive Value
	if naiveLoss, err := computeLoss(naiveValues, "naive value"); err != nil {
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
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.Values)
		if loss, err := computeLoss(values, fmt.Sprintf("inferer value %d", i)); err != nil {
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
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.Values)
		if loss, err := computeLoss(values, fmt.Sprintf("forecaster value %d", i)); err != nil {
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
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.CombinedInference)
		if loss, err := computeLoss(values, fmt.Sprintf("one out inferer value %d", i)); err != nil {
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
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.CombinedInference)
		if loss, err := computeLoss(values, fmt.Sprintf("one out forecaster value %d", i)); err != nil {
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
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.CombinedInference)
		if loss, err := computeLoss(values, fmt.Sprintf("one in forecaster value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for one-in forecaster value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			losses.OneInForecasterValues[i] = &emissionstypes.InputWorkerAttributedValue{Worker: val.Forecaster, Value: boundedLoss}
		}
	}

	losses.OneOutInfererForecasterValues = make([]*emissionstypes.InputOneOutInfererForecasterValues, len(vb.OneOutInfererForecasterValues))
	for i, val := range vb.OneOutInfererForecasterValues {
		oneOutInfererValues := make([]*emissionstypes.InputWithheldWorkerAttributedValue, len(vb.OneOutInfererForecasterValues)) // TODO: fix!
		values := emissionstypes.ConvertLabeledValuesToDecArray(val.CombinedInference)
		if loss, err := computeLoss(values, fmt.Sprintf("one out inferer value %d", i)); err != nil {
			return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error computing loss for one-out inferer value")
		} else {
			boundedLoss, err := alloraMath.NewBoundedExp40Dec(loss)
			if err != nil {
				return emissionstypes.InputValueBundle{}, errorsmod.Wrapf(err, "error converting naive loss to BoundedExp40Dec")
			}
			oneOutInfererValues[i] = &emissionstypes.InputWithheldWorkerAttributedValue{Worker: val.WithheldInferer, Value: boundedLoss} // TODO: fix!
		}

		losses.OneOutInfererForecasterValues[i] = &emissionstypes.InputOneOutInfererForecasterValues{
			Forecaster:          val.Forecaster,
			OneOutInfererValues: oneOutInfererValues,
		}
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
