package usecase

import (
	"allora_offchain_node/lib"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/rs/zerolog/log"
)

func NewNegativeInfinity() alloraMath.Dec {
	// var dec apd.Decimal
	// dec.Negative = true
	// dec.Form = apd.Infinite
	// return alloraMath.Dec{dec: dec, isNaN: false}
	dec, err := alloraMath.MustNewDecFromString("1").Quo(alloraMath.MustNewDecFromString("0"))
	if err != nil {
		log.Error().Err(err).Msg("Error creating negative infinity")
	}
	return dec
}

// Get the reputer's values at the block from the chain
// Compute loss bundle with the reputer provided Loss function and ground truth
// sign and commit to chain
func (suite *UseCaseSuite) BuildCommitReputerPayload(reputer lib.ReputerConfig, nonce lib.BlockHeight) (bool, error) {
	ctx := context.Background()

	valueBundle, err := suite.Node.GetReputerValuesAtBlock(reputer.TopicId, nonce)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to get reputer values at block")
		return false, err
	}
	valueBundle.ReputerRequestNonce = &emissionstypes.ReputerRequestNonce{
		ReputerNonce: &emissionstypes.Nonce{BlockHeight: nonce},
	}
	valueBundle.Reputer = suite.Node.Wallet.Address

	sourceTruth, err := reputer.ReputerEntrypoint.SourceTruth(reputer, nonce)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to get source truth from reputer")
		return false, err
	}
	suite.Metrics.IncrementMetricsCounter(lib.TruthRequestCount, suite.Node.Chain.Address, reputer.TopicId)

	lossBundle, err := suite.ComputeLossBundle(sourceTruth, valueBundle, reputer)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to compute loss bundle")
		return false, err
	}
	suite.Metrics.IncrementMetricsCounter(lib.ReputerDataBuildCount, suite.Node.Chain.Address, reputer.TopicId)

	signedValueBundle, err := suite.SignReputerValueBundle(&lossBundle)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to sign reputer value bundle")
		return false, err
	}

	req := &emissionstypes.InsertReputerPayloadRequest{
		Sender:             suite.Node.Wallet.Address,
		ReputerValueBundle: signedValueBundle,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msgf("Error marshaling MsgInserReputerPayload to print Msg as JSON")
	} else {
		log.Debug().Uint64("topicId", reputer.TopicId).Msgf("Sending InsertReputerPayload to chain %s", string(reqJSON))
	}
	if suite.Node.Wallet.SubmitTx {
		_, err = suite.Node.SendDataWithRetry(ctx, req, "Send Reputer Data to chain")
		if err != nil {
			log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msgf("Error sending Reputer Data to chain: %s", err)
			return false, err
		}
		suite.Metrics.IncrementMetricsCounter(lib.ReputerChainSubmissionCount, suite.Node.Chain.Address, reputer.TopicId)
	} else {
		log.Info().Uint64("topicId", reputer.TopicId).Msg("SubmitTx=false; Skipping sending Reputer Data to chain")
	}

	return true, nil
}

func (suite *UseCaseSuite) ComputeLossBundle(sourceTruth string, vb *emissionstypes.ValueBundle, reputer lib.ReputerConfig) (emissionstypes.ValueBundle, error) {
	if vb == nil {
		return emissionstypes.ValueBundle{}, errors.New("nil ValueBundle")
	}
	// Check if vb is empty
	if IsEmpty(*vb) {
		return emissionstypes.ValueBundle{}, errors.New("empty ValueBundle")
	}
	if err := ValidateDec(vb.CombinedValue); err != nil {
		return emissionstypes.ValueBundle{}, errors.New("ValueBundle - invalid CombinedValue")
	}
	if err := ValidateDec(vb.NaiveValue); err != nil {
		return emissionstypes.ValueBundle{}, errors.New("ValueBundle - invalid NaiveValue")
	}

	losses := emissionstypes.ValueBundle{
		TopicId:             vb.TopicId,
		ReputerRequestNonce: vb.ReputerRequestNonce,
		Reputer:             vb.Reputer,
		ExtraData:           vb.ExtraData,
	}

	computeLoss := func(value alloraMath.Dec, description string) (alloraMath.Dec, error) {
		lossStr, err := reputer.ReputerEntrypoint.LossFunction(sourceTruth, value.String())
		if err != nil {
			return alloraMath.Dec{}, fmt.Errorf("error computing loss for %s: %w", description, err)
		}

		loss, err := alloraMath.NewDecFromString(lossStr)
		if err != nil {
			return alloraMath.Dec{}, fmt.Errorf("error parsing loss value for %s: %w", description, err)
		}

		if !reputer.AllowsNegativeValue {
			loss, err = alloraMath.Log10(loss)
			if err != nil {
				return alloraMath.Dec{}, fmt.Errorf("error Log10 for %s: %w", description, err)
			}
		}

		if err := ValidateDec(loss); err != nil {
			return alloraMath.Dec{}, fmt.Errorf("invalid loss value for %s: %w", description, err)
		}

		return loss, nil
	}

	// Combined Value
	if combinedLoss, err := computeLoss(vb.CombinedValue, "combined value"); err != nil {
		log.Error().Err(err).Msg("Error computing loss for combined value")
		return emissionstypes.ValueBundle{}, err
	} else {
		losses.CombinedValue = combinedLoss
	}

	// Naive Value
	if naiveLoss, err := computeLoss(vb.NaiveValue, "naive value"); err != nil {
		log.Error().Err(err).Msg("Error computing loss for naive value")
		return emissionstypes.ValueBundle{}, err
	} else {
		losses.NaiveValue = naiveLoss
	}

	// Inferer Values
	losses.InfererValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.InfererValues))
	for i, val := range vb.InfererValues {
		if loss, err := computeLoss(val.Value, fmt.Sprintf("inferer value %d", i)); err != nil {
			log.Error().Err(err).Msg("Error computing loss for inferer value")
			return emissionstypes.ValueBundle{}, err
		} else {
			losses.InfererValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
		}
	}

	// Forecaster Values
	losses.ForecasterValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.ForecasterValues))
	for i, val := range vb.ForecasterValues {
		if loss, err := computeLoss(val.Value, fmt.Sprintf("forecaster value %d", i)); err != nil {
			log.Error().Err(err).Msg("Error computing loss for forecaster value")
			return emissionstypes.ValueBundle{}, err
		} else {
			losses.ForecasterValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
		}
	}

	// One Out Inferer Values
	losses.OneOutInfererValues = make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutInfererValues))
	for i, val := range vb.OneOutInfererValues {
		if loss, err := computeLoss(val.Value, fmt.Sprintf("one out inferer value %d", i)); err != nil {
			log.Error().Err(err).Msg("Error computing loss for one out inferer value")
			return emissionstypes.ValueBundle{}, err
		} else {
			losses.OneOutInfererValues[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
		}
	}

	// One Out Forecaster Values
	losses.OneOutForecasterValues = make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutForecasterValues))
	for i, val := range vb.OneOutForecasterValues {
		if loss, err := computeLoss(val.Value, fmt.Sprintf("one out forecaster value %d", i)); err != nil {
			log.Error().Err(err).Msg("Error computing loss for one out forecaster value")
			return emissionstypes.ValueBundle{}, err
		} else {
			losses.OneOutForecasterValues[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: loss}
		}
	}

	// One In Forecaster Values
	losses.OneInForecasterValues = make([]*emissionstypes.WorkerAttributedValue, len(vb.OneInForecasterValues))
	for i, val := range vb.OneInForecasterValues {
		if loss, err := computeLoss(val.Value, fmt.Sprintf("one in forecaster value %d", i)); err != nil {
			log.Error().Err(err).Msg("Error computing loss for one in forecaster value")
			return emissionstypes.ValueBundle{}, err
		} else {
			losses.OneInForecasterValues[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: loss}
		}
	}
	return losses, nil
}

func (suite *UseCaseSuite) SignReputerValueBundle(valueBundle *emissionstypes.ValueBundle) (*emissionstypes.ReputerValueBundle, error) {
	// Marshall and sign the bundle
	protoBytesIn := make([]byte, 0) // Create a byte slice with initial length 0 and capacity greater than 0
	protoBytesIn, err := valueBundle.XXX_Marshal(protoBytesIn, true)
	if err != nil {
		log.Error().Err(err).Msg("Error Marshalling valueBundle")
		return &emissionstypes.ReputerValueBundle{}, err
	}
	sig, pk, err := suite.Node.Chain.Client.Context().Keyring.Sign(suite.Node.Chain.Account.Name, protoBytesIn, signing.SignMode_SIGN_MODE_DIRECT)
	pkStr := hex.EncodeToString(pk.Bytes())
	if err != nil {
		log.Error().Err(err).Msg("Error signing valueBundle")
		return &emissionstypes.ReputerValueBundle{}, err
	}

	reputerValueBundle := &emissionstypes.ReputerValueBundle{
		ValueBundle: valueBundle,
		Signature:   sig,
		Pubkey:      pkStr,
	}

	return reputerValueBundle, nil
}
