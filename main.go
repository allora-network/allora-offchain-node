package main

import (
	"allora_offchain_node/lib"
	usecase "allora_offchain_node/usecase"
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func ConvertEntrypointsToInstances(userConfig lib.UserConfig) error {
	/// Initialize adapters using the factory function
	for i, worker := range userConfig.Worker {
		if worker.InferenceEntrypointName != "" {
			adapter, err := NewAlloraAdapter(worker.InferenceEntrypointName)
			if err != nil {
				fmt.Println("Error creating inference adapter:", err)
				return err
			}
			userConfig.Worker[i].InferenceEntrypoint = adapter
		}

		if worker.ForecastEntrypointName != "" {
			adapter, err := NewAlloraAdapter(worker.ForecastEntrypointName)
			if err != nil {
				fmt.Println("Error creating forecast adapter:", err)
				return err
			}
			userConfig.Worker[i].ForecastEntrypoint = adapter
		}
	}

	for i, reputer := range userConfig.Reputer {
		if reputer.ReputerEntrypointName != "" {
			adapter, err := NewAlloraAdapter(reputer.ReputerEntrypointName)
			if err != nil {
				fmt.Println("Error creating reputer adapter:", err)
				return err
			}
			userConfig.Reputer[i].ReputerEntrypoint = adapter
		}
	}
	return nil
}

func main() {
	if dotErr := godotenv.Load(); dotErr != nil {
		log.Info().Msg("Unable to load .env file")
	}

	// UNIX Time is faster and smaller than most timestamps
	// zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Info().Msg("Starting allora offchain node...")

	metrics := lib.NewMetrics(lib.COUNTER_DATA)
	metrics.RegisterMetricsCounters()
	metrics.StartMetricsServer(":2112")

	finalUserConfig := lib.UserConfig{}
	alloraJsonConfig := os.Getenv(lib.ALLORA_OFFCHAIN_NODE_CONFIG_JSON)
	if alloraJsonConfig != "" {
		log.Info().Msg("Config using JSON env var")
		// completely reset UserConfig
		err := json.Unmarshal([]byte(alloraJsonConfig), &finalUserConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse JSON config file from Config")
			return
		}
	} else if os.Getenv(lib.ALLORA_OFFCHAIN_NODE_CONFIG_FILE_PATH) != "" {
		log.Info().Msg("Config using JSON config file")
		// parse file defined in CONFIG_FILE_PATH into UserConfig
		file, err := os.Open(os.Getenv(lib.ALLORA_OFFCHAIN_NODE_CONFIG_FILE_PATH))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to open JSON config file")
			return
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		// completely reset UserConfig
		err = decoder.Decode(&finalUserConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse JSON config file")
			return
		}
	} else {
		log.Fatal().Msg("Could not find config file. Please create a config.json file and pass as environment variable.")
		return
	}

	// Convert entrypoints to instances of adapters
	err := ConvertEntrypointsToInstances(finalUserConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to convert Entrypoints to instances of adapters")
		return
	}
	spawner, err := usecase.NewUseCaseSuite(finalUserConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize use case, exiting")
		return
	}

	spawner.Metrics = *metrics
	spawner.Spawn()
}
