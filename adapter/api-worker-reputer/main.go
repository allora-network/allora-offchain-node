package api_worker_reputer

import (
	"allora_offchain_node/lib"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

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

// Expects forecast as a json array of NodeValue
func (a *AlloraAdapter) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	urlTemplate := node.Parameters["InferenceEndpoint"]
	url := replaceExtendedPlaceholders(urlTemplate, node.Parameters, blockHeight, node.TopicId)
	log.Debug().Str("url", url).Msg("Inference")
	forecastsAsString, err := requestEndpoint(url)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get forecasts")
		return []lib.NodeValue{}, err
	}
	// parse json forecasts into a slice of NodeValue
	var nodeValues []lib.NodeValue
	err = json.Unmarshal([]byte(forecastsAsString), &nodeValues)
	if err != nil {
		log.Error().Err(err).Msg("Error unmarshalling JSON forecasts")
	}
	return []lib.NodeValue{}, nil
}

func (a *AlloraAdapter) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	urlTemplate := node.Parameters["SourceOfTruthEndpoint"]
	url := replaceExtendedPlaceholders(urlTemplate, node.Parameters, blockHeight, node.TopicId)
	log.Debug().Str("url", url).Msg("Source of truth")
	return requestEndpoint(url)
}

func (a *AlloraAdapter) LossFunction(sourceTruth string, inferenceValue string) string {
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
