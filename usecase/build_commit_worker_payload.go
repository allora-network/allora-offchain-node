package usecase

import (
	"allora_offchain_node/lib"
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/rs/zerolog/log"

	alloraMath "github.com/allora-network/allora-chain/math"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

func (suite *UseCaseSuite) BuildCommitWorkerPayload(ctx context.Context, worker lib.WorkerConfig, nonce *emissionstypes.Nonce) (bool, error) {
	if worker.InferenceEntrypoint == nil && worker.ForecastEntrypoint == nil {
		log.Error().Msg("Worker has no valid Inference or Forecast entrypoints")
		return false, nil
	}

	var workerResponse = lib.WorkerResponse{
		WorkerConfig: worker,
	}

	if worker.InferenceEntrypoint != nil {
		inference, err := worker.InferenceEntrypoint.CalcInference(worker, nonce.BlockHeight)
		if err != nil {
			log.Error().Err(err).Str("worker", worker.InferenceEntrypoint.Name()).Msg("Error computing inference for worker")
			return false, err
		}
		workerResponse.InfererValue = inference
		suite.Metrics.IncrementMetricsCounter(lib.InferenceRequestCount, suite.Node.Chain.Address, worker.TopicId)
	}

	if worker.ForecastEntrypoint != nil {
		forecasts := []lib.NodeValue{}
		forecasts, err := worker.ForecastEntrypoint.CalcForecast(worker, nonce.BlockHeight)
		if err != nil {
			log.Error().Err(err).Str("worker", worker.ForecastEntrypoint.Name()).Msg("Error computing forecast for worker")
			return false, err
		}
		workerResponse.ForecasterValues = forecasts
		suite.Metrics.IncrementMetricsCounter(lib.ForecastRequestCount, suite.Node.Chain.Address, worker.TopicId)
	}

	workerPayload, err := suite.BuildWorkerPayload(workerResponse, nonce.BlockHeight)
	if err != nil {
		log.Error().Err(err).Msg("Error building workerPayload")
		return false, err
	}
	suite.Metrics.IncrementMetricsCounter(lib.WorkerDataBuildCount, suite.Node.Chain.Address, worker.TopicId)

	workerDataBundle, err := suite.SignWorkerPayload(&workerPayload)
	if err != nil {
		log.Error().Err(err).Msg("Error signing workerPayload")
		return false, err
	}
	workerDataBundle.Nonce = nonce
	workerDataBundle.TopicId = worker.TopicId

	if err := workerDataBundle.Validate(); err != nil {
		return false, err
	}

	req := &emissionstypes.InsertWorkerPayloadRequest{
		Sender:           suite.Node.Wallet.Address,
		WorkerDataBundle: workerDataBundle,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling InsertWorkerPayload to print Msg as JSON")
	} else {
		log.Info().Str("req", string(reqJSON)).Msg("Sending InsertWorkerPayload to chain")
	}

	if suite.Node.Wallet.SubmitTx {
		_, err = suite.Node.SendDataWithRetry(ctx, req, "Send Worker Data to chain")
		if err != nil {
			log.Error().Err(err).Msg("Error sending Worker Data to chain")
			return false, err
		}
		suite.Metrics.IncrementMetricsCounter(lib.WorkerChainSubmissionCount, suite.Node.Chain.Address, worker.TopicId)
	} else {
		log.Info().Uint64("topicId", worker.TopicId).Msg("SubmitTx=false; Skipping sending Worker Data to chain")
	}
	return true, nil
}

func (suite *UseCaseSuite) BuildWorkerPayload(workerResponse lib.WorkerResponse, nonce emissionstypes.BlockHeight) (emissionstypes.InferenceForecastBundle, error) {

	inferenceForecastsBundle := emissionstypes.InferenceForecastBundle{}

	if workerResponse.InfererValue != "" {
		infererValue, err := alloraMath.NewDecFromString(workerResponse.InfererValue)
		if err != nil {
			log.Error().Err(err).Msg("Error converting infererValue to Dec")
			return emissionstypes.InferenceForecastBundle{}, err
		}
		builtInference := &emissionstypes.Inference{
			TopicId:     workerResponse.TopicId,
			Inferer:     suite.Node.Wallet.Address,
			Value:       infererValue,
			BlockHeight: nonce,
		}
		inferenceForecastsBundle.Inference = builtInference
	}

	if len(workerResponse.ForecasterValues) > 0 {
		var forecasterElements []*emissionstypes.ForecastElement
		for _, val := range workerResponse.ForecasterValues {
			decVal, err := alloraMath.NewDecFromString(val.Value)
			if err != nil {
				log.Error().Err(err).Msg("Error converting forecasterValue to Dec")
				return emissionstypes.InferenceForecastBundle{}, err
			}
			if !workerResponse.AllowsNegativeValue {
				decVal, err = alloraMath.Log10(decVal)
				if err != nil {
					log.Error().Err(err).Msg("Error Log10 forecasterElements")
					return emissionstypes.InferenceForecastBundle{}, err
				}
			}
			forecasterElements = append(forecasterElements, &emissionstypes.ForecastElement{
				Inferer: val.Worker,
				Value:   decVal,
			})
		}

		if len(forecasterElements) > 0 {
			forecasterValues := &emissionstypes.Forecast{
				TopicId:          workerResponse.TopicId,
				BlockHeight:      nonce,
				Forecaster:       suite.Node.Wallet.Address,
				ForecastElements: forecasterElements,
			}
			inferenceForecastsBundle.Forecast = forecasterValues
		}
	}
	return inferenceForecastsBundle, nil
}

func (suite *UseCaseSuite) SignWorkerPayload(workerPayload *emissionstypes.InferenceForecastBundle) (*emissionstypes.WorkerDataBundle, error) {
	// Marshall and sign the bundle
	protoBytesIn := make([]byte, 0) // Create a byte slice with initial length 0 and capacity greater than 0
	protoBytesIn, err := workerPayload.XXX_Marshal(protoBytesIn, true)
	if err != nil {
		log.Error().Err(err).Msg("Error Marshalling workerPayload")
		return &emissionstypes.WorkerDataBundle{}, err
	}
	sig, pk, err := suite.Node.Chain.Client.Context().Keyring.Sign(suite.Node.Chain.Account.Name, protoBytesIn, signing.SignMode_SIGN_MODE_DIRECT)
	pkStr := hex.EncodeToString(pk.Bytes())
	if err != nil {
		log.Error().Err(err).Msg("Error signing the InferenceForecastsBundle message")
		return &emissionstypes.WorkerDataBundle{}, err
	}
	// Create workerDataBundle with signature
	workerDataBundle := &emissionstypes.WorkerDataBundle{
		Worker:                             suite.Node.Wallet.Address,
		InferenceForecastsBundle:           workerPayload,
		InferencesForecastsBundleSignature: sig,
		Pubkey:                             pkStr,
	}

	return workerDataBundle, nil
}
