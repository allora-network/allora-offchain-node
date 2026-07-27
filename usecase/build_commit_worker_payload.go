package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"allora_offchain_node/lib"
	"allora_offchain_node/lib/auth"
	"allora_offchain_node/metrics"

	errorsmod "cosmossdk.io/errors"
	"github.com/rs/zerolog/log"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
)

// multiLabel is resolved once at startup from the topic's immutable on-chain
// OutputArity (see startWorker) and threaded in, rather than re-queried per
// submission.
func (suite *UseCaseSuite) BuildCommitWorkerPayload(ctx context.Context, worker lib.WorkerConfig, nonce emissionstypes.Nonce, timeoutHeight uint64, multiLabel bool) error {
	log := log.With().Uint64("topicId", worker.TopicId).Str("actorType", "worker").Logger()
	log.Info().Msg("Building worker payload")

	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet")
	}
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet config")
	}

	if worker.InferenceEntrypoint == nil && worker.ForecastEntrypoint == nil {
		return errors.New("Worker has no valid Inference or Forecast entrypoints")
	}

	workerResponse, err := suite.getWorkerResponse(worker, multiLabel, nonce.BlockHeight, wallet.Address)
	if err != nil {
		return err
	}

	workerPayload, err := suite.BuildWorkerPayload(workerResponse, nonce.BlockHeight)
	if err != nil {
		return errorsmod.Wrapf(err, "Error building worker payload, topicId: %d, blockHeight: %d", worker.TopicId, nonce.BlockHeight)
	}

	workerDataBundle, err := suite.SignWorkerPayload(&workerPayload)
	if err != nil {
		return errorsmod.Wrapf(err, "Error signing worker payload, topicId: %d, blockHeight: %d", worker.TopicId, nonce.BlockHeight)
	}
	workerDataBundle.Nonce = &nonce
	workerDataBundle.TopicId = worker.TopicId

	if err := workerDataBundle.Validate(); err != nil {
		return errorsmod.Wrapf(err, "Error validating worker data bundle, topicId: %d, blockHeight: %d", worker.TopicId, nonce.BlockHeight)
	}

	req := &emissionstypes.InsertWorkerPayloadRequest{
		Sender:           wallet.Address,
		WorkerDataBundle: workerDataBundle,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Warn().Err(err).Msg("Error marshaling InsertWorkerPayload to print Msg as JSON")
	} else {
		log.Info().Str("req", string(reqJSON)).Msg("Sending InsertWorkerPayload to chain")
	}

	if walletConfig.SubmitTx {
		_, err = suite.ConnectionManager.SendDataWithNodeRetry(ctx, req, timeoutHeight, "Send Worker Data to chain")
		if err != nil {
			return errorsmod.Wrapf(err, "Error sending Worker Data to chain, topicId: %d, blockHeight: %d", worker.TopicId, nonce.BlockHeight)
		}
		suite.Metrics.IncrementMetricsCounter(metrics.WorkerChainSubmissionCount, wallet.Address, worker.TopicId)
	} else {
		log.Info().Msg("SubmitTx=false; Skipping sending Worker Data to chain")
	}
	return nil
}

// getWorkerResponse gathers the worker's inference and forecast payloads from the
// configured entrypoints. The inference path dispatches on the topic's on-chain
// output arity (multiLabel): a multi-label topic fetches a labeled inference, a
// single-label topic fetches a scalar one. If the endpoint required for the
// topic's arity is missing it errors; if a contradicting endpoint is also present
// it warns and ignores it.
func (suite *UseCaseSuite) getWorkerResponse(worker lib.WorkerConfig, multiLabel bool, blockHeight int64, walletAddress string) (lib.WorkerResponse, error) {
	log := log.With().Uint64("topicId", worker.TopicId).Str("actorType", "worker").Logger()
	workerResponse := lib.WorkerResponse{ //nolint:exhaustruct
		WorkerConfig: worker,
	}

	if worker.InferenceEntrypoint != nil {
		_, hasLabeled := worker.Parameters[lib.ParamLabeledInferenceEndpoint]
		_, hasScalar := worker.Parameters[lib.ParamInferenceEndpoint]
		if multiLabel {
			if !hasLabeled {
				return lib.WorkerResponse{}, errorsmod.Wrapf(emissionstypes.ErrInvalidValue, //nolint:exhaustruct
					"topic %d is multi-label (MULTI) but no %s is configured", worker.TopicId, lib.ParamLabeledInferenceEndpoint)
			}
			if hasScalar {
				log.Warn().Msgf("topic is multi-label but %s is also configured; ignoring it", lib.ParamInferenceEndpoint)
			}
			labeledInference, err := worker.InferenceEntrypoint.CalcLabeledInference(worker, blockHeight)
			if err != nil {
				return lib.WorkerResponse{}, errorsmod.Wrapf(err, "Error computing labeled inference for worker, topicId: %d, blockHeight: %d", worker.TopicId, blockHeight) //nolint:exhaustruct
			}
			workerResponse.InfererValues = labeledInference
			suite.Metrics.IncrementMetricsCounter(metrics.LabeledInferenceRequestCount, walletAddress, worker.TopicId)
		} else {
			if !hasScalar {
				return lib.WorkerResponse{}, errorsmod.Wrapf(emissionstypes.ErrInvalidValue, //nolint:exhaustruct
					"topic %d is single-label (SINGLE) but no %s is configured", worker.TopicId, lib.ParamInferenceEndpoint)
			}
			if hasLabeled {
				log.Warn().Msgf("topic is single-label but %s is also configured; ignoring it", lib.ParamLabeledInferenceEndpoint)
			}
			inference, err := worker.InferenceEntrypoint.CalcInference(worker, blockHeight)
			if err != nil {
				return lib.WorkerResponse{}, errorsmod.Wrapf(err, "Error computing inference for worker, topicId: %d, blockHeight: %d", worker.TopicId, blockHeight) //nolint:exhaustruct
			}
			workerResponse.InfererValue = inference
			suite.Metrics.IncrementMetricsCounter(metrics.InferenceRequestCount, walletAddress, worker.TopicId)
		}
	}

	if worker.ForecastEntrypoint != nil {
		forecasts, err := worker.ForecastEntrypoint.CalcForecast(worker, blockHeight)
		if err != nil {
			return lib.WorkerResponse{}, errorsmod.Wrapf(err, "Error computing forecast for worker, topicId: %d, blockHeight: %d", worker.TopicId, blockHeight) //nolint:exhaustruct
		}
		workerResponse.ForecasterValues = forecasts
		suite.Metrics.IncrementMetricsCounter(metrics.ForecastRequestCount, walletAddress, worker.TopicId)
	}

	return workerResponse, nil
}

func (suite *UseCaseSuite) BuildWorkerPayload(workerResponse lib.WorkerResponse, nonce emissionstypes.BlockHeight) (emissionstypes.InputInferenceForecastBundle, error) {
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return emissionstypes.InputInferenceForecastBundle{}, errorsmod.Wrapf(err, "error getting wallet") //nolint:exhaustruct
	}

	inferenceForecastsBundle := emissionstypes.InputInferenceForecastBundle{} //nolint:exhaustruct

	// Populate exactly one of Value / Values, keyed on whether the worker produced
	// a multi-label (Values) or scalar (Value) inference. The chain treats Values
	// as authoritative for multi-label topics; setting a spurious zero Value on a
	// labeled submission would be recorded as a scalar inference by any consumer
	// reading Value (e.g. a not-yet-upgraded validator), so the two are kept
	// mutually exclusive here.
	switch {
	case len(workerResponse.InfererValues) > 0:
		infererValues := make([]*emissionstypes.InputLabeledValue, len(workerResponse.InfererValues))
		for i := range workerResponse.InfererValues {
			value, err := alloraMath.NewBoundedExp40DecFromString(workerResponse.InfererValues[i].Value)
			if err != nil {
				return emissionstypes.InputInferenceForecastBundle{}, errorsmod.Wrapf(err, "error converting infererValues to Dec") //nolint:exhaustruct
			}
			infererValues[i] = &emissionstypes.InputLabeledValue{
				Label: workerResponse.InfererValues[i].Label,
				Value: value,
			}
		}
		inferenceForecastsBundle.Inference = &emissionstypes.InputInference{ //nolint:exhaustruct
			TopicId:     workerResponse.TopicId,
			Inferer:     wallet.Address,
			Values:      infererValues,
			BlockHeight: nonce,
		}
	case workerResponse.InfererValue != "":
		infererValue, err := alloraMath.NewBoundedExp40DecFromString(workerResponse.InfererValue)
		if err != nil {
			return emissionstypes.InputInferenceForecastBundle{}, errorsmod.Wrapf(err, "error converting infererValue to Dec") //nolint:exhaustruct
		}
		inferenceForecastsBundle.Inference = &emissionstypes.InputInference{ //nolint:exhaustruct
			TopicId:     workerResponse.TopicId,
			Inferer:     wallet.Address,
			Value:       infererValue,
			BlockHeight: nonce,
		}
	}

	if len(workerResponse.ForecasterValues) > 0 {
		var forecasterElements []*emissionstypes.InputForecastElement //nolint:exhaustruct
		for _, val := range workerResponse.ForecasterValues {
			decVal, err := alloraMath.NewBoundedExp40DecFromString(val.Value)
			if err != nil {
				return emissionstypes.InputInferenceForecastBundle{}, errorsmod.Wrapf(err, "error converting forecasterValue to Dec") //nolint:exhaustruct
			}
			forecasterElements = append(forecasterElements, &emissionstypes.InputForecastElement{
				Inferer: val.Worker,
				Value:   decVal,
			})
		}

		if len(forecasterElements) > 0 {
			forecasterValues := &emissionstypes.InputForecast{ //nolint:exhaustruct
				TopicId:          workerResponse.TopicId,
				BlockHeight:      nonce,
				Forecaster:       wallet.Address,
				ForecastElements: forecasterElements,
				ExtraData:        nil,
			}
			inferenceForecastsBundle.Forecast = forecasterValues
		}
	}
	return inferenceForecastsBundle, nil
}

func (suite *UseCaseSuite) SignWorkerPayload(workerPayload *emissionstypes.InputInferenceForecastBundle) (*emissionstypes.InputWorkerDataBundle, error) {
	// Marshal and sign the bundle
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return &emissionstypes.InputWorkerDataBundle{}, errorsmod.Wrapf(err, "error getting wallet") //nolint:exhaustruct
	}
	sig, pk, err := auth.MarshalAndSignByPrivKey(workerPayload, wallet.GetPrivKey(), wallet.AddressSDK)
	if err != nil {
		return &emissionstypes.InputWorkerDataBundle{}, errorsmod.Wrapf(err, "error signing the InferenceForecastsBundle message") //nolint:exhaustruct
	}
	pkStr := hex.EncodeToString(pk)
	// Create workerDataBundle with signature
	workerDataBundle := &emissionstypes.InputWorkerDataBundle{ //nolint:exhaustruct
		Worker:                             wallet.Address,
		InferenceForecastsBundle:           workerPayload,
		InferenceForecastsBundleSignature: sig,
		Pubkey:                             pkStr,
	}

	return workerDataBundle, nil
}
