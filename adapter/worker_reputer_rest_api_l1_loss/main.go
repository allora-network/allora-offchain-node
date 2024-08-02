package worker_reputer_rest_api_l1_loss

import (
	"allora_offchain_node/lib"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

type AlloraAdapter struct {
	name string
}

func (a *AlloraAdapter) Name() string {
	return a.name
}

func requestLocalEndpoint(url string) (string, error) {
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

func (a *AlloraAdapter) CalcInference(node lib.WorkerConfig, blockHeight int64) (string, error) {
	urlBase := node.Parameters["inferenceEndpoint"]
	token := node.Parameters["token"]
	url := fmt.Sprintf("%s/%s", urlBase, token)
	return requestLocalEndpoint(url)
}

func (a *AlloraAdapter) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.NodeValue{}, nil
}

func (a *AlloraAdapter) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
	urlBase := node.Parameters["truthEndpoint"]
	token := node.Parameters["token"]
	url := fmt.Sprintf("%s/%s", urlBase, token)
	return requestLocalEndpoint(url)
}

func (a *AlloraAdapter) LossFunction(sourceTruth string, inferenceValue string) string {
	log.Debug().Str("name", a.name).Msg("Loss function processing")
	sourceTruthFloat, _ := strconv.ParseFloat(sourceTruth, 64)
	inferenceValueFloat, _ := strconv.ParseFloat(inferenceValue, 64)
	loss := math.Abs(sourceTruthFloat - inferenceValueFloat)
	str := fmt.Sprintf("%f", loss)
	log.Debug().Str("str", str).Msg("Returned loss value")
	return str
}

func (a *AlloraAdapter) CanInfer() bool {
	return true
}

func (a *AlloraAdapter) CanForecast() bool {
	return false
}

func (a *AlloraAdapter) CanSourceTruthAndComputeLoss() bool {
	return true
}

func NewAlloraAdapter() *AlloraAdapter {
	return &AlloraAdapter{
		name: "worker_reputer_rest_api_l1_loss",
	}
}
