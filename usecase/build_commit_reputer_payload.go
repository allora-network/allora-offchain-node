package usecase

import (
	"allora_offchain_node/lib"
	"context"
	"encoding/hex"
	"encoding/json"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/rs/zerolog/log"
)

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

	lossBundle, err := suite.ComputeLossBundle(sourceTruth, valueBundle, reputer)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to compute loss bundle")
		return false, err
	}

	signedValueBundle, err := suite.SignReputerValueBundle(&lossBundle)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to sign reputer value bundle")
		return false, err
	}

	req := &emissionstypes.MsgInsertReputerPayload{
		Sender:             suite.Node.Wallet.Address,
		ReputerValueBundle: signedValueBundle,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msgf("Error marshaling MsgInserReputerPayload to print Msg as JSON")
	} else {
		log.Debug().Uint64("topicId", reputer.TopicId).Msgf("Sending MsgInsertReputerPayload to chain %s", string(reqJSON))
	}
	_, err = suite.Node.SendDataWithRetry(ctx, req, "Send Reputer Data to chain")
	if err != nil {
		log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msgf("Error sending Reputer Data to chain: %s", err)
		return false, err
	}

	return true, nil
}

func (suite *UseCaseSuite) ComputeLossBundle(sourceTruth string, vb *emissionstypes.ValueBundle, reputer lib.ReputerConfig) (emissionstypes.ValueBundle, error) {
	losses := emissionstypes.ValueBundle{
		TopicId:             vb.TopicId,
		ReputerRequestNonce: vb.ReputerRequestNonce,
		Reputer:             vb.Reputer,
		ExtraData:           vb.ExtraData,
	}

	if combineValueLoss, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, vb.CombinedValue.Abs().String())); err != nil {
		log.Error().Err(err).Msg("Error computing loss for combined value")
		return emissionstypes.ValueBundle{}, err
	} else {
		if !reputer.AllowsNegativeValue {
			combineValueLoss, err = alloraMath.Log10(combineValueLoss)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for Combined Value:")
			}
		}
		losses.CombinedValue = combineValueLoss
	}

	if naiveValue, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, vb.NaiveValue.Abs().String())); err != nil {
		log.Error().Err(err).Msg("Error computing loss for naive value")
		return emissionstypes.ValueBundle{}, err
	} else {
		if !reputer.AllowsNegativeValue {
			naiveValue, err = alloraMath.Log10(naiveValue)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for Naive Value:")
			}
		}
		losses.NaiveValue = naiveValue
	}

	infererLosses := make([]*emissionstypes.WorkerAttributedValue, len(vb.InfererValues))
	for i, val := range vb.InfererValues {
		value, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, val.Value.Abs().String()))
		if err != nil {
			log.Error().Err(err).Msg("Error computing loss for inferer values")
			return emissionstypes.ValueBundle{}, err
		}
		if !reputer.AllowsNegativeValue {
			value, err = alloraMath.Log10(value)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for inferer Values")
			}
		}
		infererLosses[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: value}
	}
	losses.InfererValues = infererLosses

	forecasterLosses := make([]*emissionstypes.WorkerAttributedValue, len(vb.ForecasterValues))
	for i, val := range vb.ForecasterValues {
		value, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, val.Value.Abs().String()))
		if err != nil {
			log.Error().Err(err).Msg("Error computing loss for forecaster values")
			return emissionstypes.ValueBundle{}, err
		}
		if !reputer.AllowsNegativeValue {
			value, err = alloraMath.Log10(value)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for forecaster Values")
			}
		}
		forecasterLosses[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: value}
	}
	losses.ForecasterValues = forecasterLosses

	oneOutInfererLosses := make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutInfererValues))
	for i, val := range vb.OneOutInfererValues {
		value, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, val.Value.Abs().String()))
		if err != nil {
			log.Error().Err(err).Msg("Error computing loss for one out inferer values")
			return emissionstypes.ValueBundle{}, err
		}
		if !reputer.AllowsNegativeValue {
			value, err = alloraMath.Log10(value)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for out inferer values")
			}
		}
		oneOutInfererLosses[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: value}
	}
	losses.OneOutInfererValues = oneOutInfererLosses

	oneOutForecasterLosses := make([]*emissionstypes.WithheldWorkerAttributedValue, len(vb.OneOutForecasterValues))
	for i, val := range vb.OneOutForecasterValues {
		value, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, val.Value.Abs().String()))
		if err != nil {
			log.Error().Err(err).Msg("Error computing loss for one out forecaster values")
			return emissionstypes.ValueBundle{}, err
		}
		if !reputer.AllowsNegativeValue {
			value, err = alloraMath.Log10(value)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for out forecaster values")
			}
		}
		oneOutForecasterLosses[i] = &emissionstypes.WithheldWorkerAttributedValue{Worker: val.Worker, Value: value}
	}
	losses.OneOutForecasterValues = oneOutForecasterLosses

	oneInForecasterLosses := make([]*emissionstypes.WorkerAttributedValue, len(vb.OneInForecasterValues))
	for i, val := range vb.OneInForecasterValues {
		value, err := alloraMath.NewDecFromString(reputer.ReputerEntrypoint.LossFunction(sourceTruth, val.Value.Abs().String()))
		if err != nil {
			log.Error().Err(err).Msg("Error computing loss for one in forecaster values")
			return emissionstypes.ValueBundle{}, err
		}
		if !reputer.AllowsNegativeValue {
			value, err = alloraMath.Log10(value)
			if err != nil {
				log.Error().Err(err).Msg("Error Log10 for in forecaster values")
			}
		}
		oneInForecasterLosses[i] = &emissionstypes.WorkerAttributedValue{Worker: val.Worker, Value: value}
	}
	losses.OneInForecasterValues = oneInForecasterLosses
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
