package worker_coin_predictor_10min

import (
	"allora_offchain_node/lib"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog/log"
)

type Config struct {
	InferenceEndpoint string `json:"inferenceEndpoint"`
	Token             string `json:"token"`
	ForecastEndpoint  string `json:"forecastEndpoint"`
}

type AlloraEntrypoint struct {
	name   string
	config Config
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference(node lib.WorkerConfig, blockHeight int64) (string, error) {
	return "3000", nil
	// url := fmt.Sprintf("%s/%s", a.config.InferenceEndpoint, a.config.Token)
	// // make request to url
	// resp, err := http.Get(url)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to make request to %s: %w", url, err)
	// }
	// defer resp.Body.Close()

	// // Check if the response status is OK
	// if resp.StatusCode != http.StatusOK {
	// 	return "", fmt.Errorf("received non-OK HTTP status %d", resp.StatusCode)
	// }

	// // Read the response body
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to read response body: %w", err)
	// }

	// log.Debug().Bytes("body", body).Msg("Inference")
	// // convert bytes to string
	// return string(body), nil
}

func (a *AlloraEntrypoint) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.NodeValue{}, nil
}

func (a *AlloraEntrypoint) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
	return "3500.00", nil
}

func (a *AlloraEntrypoint) LossFunction(sourceTruth string, inferenceValue string) string {
	log.Debug().Str("name", a.name).Msg("Loss function processing")
	sourceTruthFloat, _ := strconv.ParseFloat(sourceTruth, 64)
	inferenceValueFloat, _ := strconv.ParseFloat(inferenceValue, 64)
	loss := math.Abs(sourceTruthFloat - inferenceValueFloat)
	str := fmt.Sprintf("%f", loss)
	log.Debug().Str("str", str).Msg("Returned loss value")
	return str
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return true
}

func (a *AlloraEntrypoint) CanSourceTruth() bool {
	return true
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	name := "worker_coin_predictor_10min_eth"

	// Get the current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get current working directory")
	}

	// Construct the path to the config file
	fileSubPath := filepath.Join("pkg", name, "config.json")
	configFilePath := filepath.Join(workingDir, fileSubPath)

	configFile, err := os.Open(configFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open config file")
	}
	defer configFile.Close()

	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	var config Config
	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config file")
	}

	return &AlloraEntrypoint{
		name:   name,
		config: config,
	}
}
