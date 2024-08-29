package api_worker_reputer

import (
	"allora_offchain_node/lib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	alloraMath "github.com/allora-network/allora-chain/math"
	"github.com/rs/zerolog/log"
)

type AlloraAdapter struct {
	name string
}

func (a *AlloraAdapter) Name() string {
	return a.name
}

func replacePlaceholders(urlTemplate string, params map[string]string) string {
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		urlTemplate = strings.ReplaceAll(urlTemplate, placeholder, value)
	}
	return urlTemplate
}

// Replace placeholders and also the blockheheight
func replaceExtendedPlaceholders(urlTemplate string, params map[string]string, blockHeight int64, topicId uint64) string {
	// Create a map of default parameters
	blockHeightAsString := strconv.FormatInt(blockHeight, 10)
	topicIdAsString := strconv.FormatUint(topicId, 10)
	defaultParams := map[string]string{
		"BlockHeight": blockHeightAsString,
		"TopicId":     topicIdAsString,
	}
	urlTemplate = replacePlaceholders(urlTemplate, defaultParams)
	urlTemplate = replacePlaceholders(urlTemplate, params)
	return urlTemplate
}

func requestEndpoint(url string) (string, error) {
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

// Expects an inference as a string scalar value
func (a *AlloraAdapter) CalcInference(node lib.WorkerConfig, blockHeight int64) (string, error) {
	urlTemplate := node.Parameters["InferenceEndpoint"]
	url := replaceExtendedPlaceholders(urlTemplate, node.Parameters, blockHeight, node.TopicId)
	log.Debug().Str("url", url).Msg("Inference")
	return requestEndpoint(url)
}

// parseJSONToNodeValues parses the incoming JSON string and returns a slice of NodeValue.
func parseJSONToNodeValues(jsonStr string) ([]lib.NodeValue, error) {
	// Define a map to hold the parsed JSON data.
	var data map[string][]float64

	// Parse the JSON string into the map.
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return nil, err
	}

	// Create a slice to hold the NodeValues.
	var nodeValues []lib.NodeValue

	// Iterate over the map to create NodeValue structs.
	for worker, values := range data {
		if len(values) > 0 {
			// Only pick the first value in the list.
			nodeValue := lib.NodeValue{
				Worker: worker,
				Value:  fmt.Sprintf("%f", values[0]),
			}
			nodeValues = append(nodeValues, nodeValue)
		}
	}

	return nodeValues, nil
}

// Expects forecast as a json array of NodeValue
func (a *AlloraAdapter) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	urlTemplate := node.Parameters["ForecastEndpoint"]
	url := replaceExtendedPlaceholders(urlTemplate, node.Parameters, blockHeight, node.TopicId)
	log.Debug().Str("url", url).Msg("Forecasts endpoint")

	forecastsAsJsonString, err := requestEndpoint(url)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get forecasts")
		return []lib.NodeValue{}, err
	}

	// parse json forecasts into a slice of NodeValue
	nodeValues, err := parseJSONToNodeValues(forecastsAsJsonString)
	if err != nil {
		log.Error().Err(err).Msg("Error transforming forecasts")
		return []lib.NodeValue{}, err
	}
	return nodeValues, nil
}

func (a *AlloraAdapter) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	urlTemplate := node.Parameters["SourceOfTruthEndpoint"]
	url := replaceExtendedPlaceholders(urlTemplate, node.Parameters, blockHeight, node.TopicId)
	log.Debug().Str("url", url).Msg("Source of truth")
	return requestEndpoint(url)
}

func (a *AlloraAdapter) LossFunction(sourceTruth string, inferenceValue string) (string, error) {
	sourceTruthDec, err := alloraMath.NewDecFromString(sourceTruth)
	if err != nil {
		return "", fmt.Errorf("failed to parse sourceTruth: %w", err)
	}

	inferenceValueDec, err := alloraMath.NewDecFromString(inferenceValue)
	if err != nil {
		return "", fmt.Errorf("failed to parse inferenceValue: %w", err)
	}

	// Calculate MSE
	diff, err := sourceTruthDec.Sub(inferenceValueDec)
	if err != nil {
		return "", fmt.Errorf("failed to calculate difference: %w", err)
	}
	squaredError, err := diff.Mul(diff)
	if err != nil {
		return "", fmt.Errorf("failed to calculate squared error: %w", err)
	}

	log.Debug().Str("MSE", squaredError.String()).Msg("Calculated MSE loss value")
	return squaredError.String(), nil
}

func (a *AlloraAdapter) CanInfer() bool {
	return true
}

func (a *AlloraAdapter) CanForecast() bool {
	return true
}

func (a *AlloraAdapter) CanSourceTruthAndComputeLoss() bool {
	return true
}

func NewAlloraAdapter() *AlloraAdapter {
	return &AlloraAdapter{
		name: "api-worker-reputer",
	}
}
