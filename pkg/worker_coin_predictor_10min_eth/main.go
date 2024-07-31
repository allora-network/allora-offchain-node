package worker_coin_predictor_10min

import (
	"allora_offchain_node/lib"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference(node lib.WorkerConfig, blockHeight int64) (string, error) {
	urlBase := node.ExtraData["inferenceEndpoint"]
	token := node.ExtraData["token"]
	url := fmt.Sprintf("%s/%s", urlBase, token)
	// make request to url
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK HTTP status %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().Bytes("body", body).Msg("Inference")
	// convert bytes to string
	return string(body), nil
}

func (a *AlloraEntrypoint) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.NodeValue{}, nil
}

func (a *AlloraEntrypoint) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
	return "", nil
}

func (a *AlloraEntrypoint) LossFunction(sourceTruth string, inferenceValue string) string {
	fmt.Println("Loss function processing" + a.name)
	sourceTruthFloat, _ := strconv.ParseFloat(sourceTruth, 64)
	inferenceValueFloat, _ := strconv.ParseFloat(inferenceValue, 64)
	loss := math.Abs(sourceTruthFloat - inferenceValueFloat)

	return fmt.Sprintf("%f", loss)
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return false
}

func (a *AlloraEntrypoint) CanSourceTruth() bool {
	return false
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "worker_coin_predictor_10min_eth",
	}
}
