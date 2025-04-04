package main

import (
	"allora_offchain_node/lib"
	"allora_offchain_node/metrics"
	usecase "allora_offchain_node/usecase"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
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
		if reputer.GroundTruthEntrypointName != "" {
			adapter, err := NewAlloraAdapter(reputer.GroundTruthEntrypointName)
			if err != nil {
				fmt.Println("Error creating reputer adapter:", err)
				return err
			}
			userConfig.Reputer[i].GroundTruthEntrypoint = adapter
		}
	}

	for i, reputer := range userConfig.Reputer {
		if reputer.LossFunctionEntrypointName != "" {
			adapter, err := NewAlloraAdapter(reputer.LossFunctionEntrypointName)
			if err != nil {
				fmt.Println("Error creating reputer adapter:", err)
				return err
			}
			userConfig.Reputer[i].LossFunctionEntrypoint = adapter
		}
	}
	return nil
}

func main() {
	// Context tree:
	// root context (rootCtx)
	// ├── essential context (essentialCtx) - for connections, wallet, workers, reputers
	// └── non-essential context (nonEssentialCtx) - for metrics, gas price updates
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// closed when the root context is cancelled in cascade
	essentialCtx, essentialCancel := context.WithCancel(rootCtx)
	nonEssentialCtx, nonEssentialCancel := context.WithCancel(rootCtx)

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info().Msg("Received shutdown signal")
		// Cancel non-essential first
		nonEssentialCancel()
		// Give some time for non-essential services to cleanup
		time.Sleep(time.Second)
		// Then cancel essential services
		essentialCancel()
		// Finally cancel root context
		rootCancel()
	}()

	// Initialize logger
	initLogger()
	if dotErr := godotenv.Load(); dotErr != nil {
		log.Info().Msg("Unable to load .env file")
	}

	// Set and lock sdk config
	config := sdktypes.GetConfig()
	config.SetBech32PrefixForAccount(lib.ADDRESS_PREFIX, lib.ADDRESS_PREFIX)
	config.Seal()

	log.Info().Msg("Starting allora offchain node...")

	// Metrics
	metrics.InitMetrics(metrics.CounterData)
	metricsServer := metrics.GetMetrics()
	metricsServer.StartMetricsServer(nonEssentialCtx, ":2112")

	// Load config and do modifications if needed
	finalUserConfig := lib.UserConfig{} // nolint: exhaustruct
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
		log.Fatal().Err(err).Msg("Failed to convert Entrypoints to instances of adapters - wrong entrypoint name?")
		return
	}

	// Check and set defaults for the user config if any values are not set
	finalUserConfig.CheckAndSetDefaults()

	// Creates the ConnectionManager and initialises the NodeConfigs with essential context
	connectionManager, err := lib.NewConnectionManager(essentialCtx, finalUserConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize ConnectionManager, exiting")
		return
	}
	defer connectionManager.Close()
	wallet, err := connectionManager.GetWallet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet, exiting")
		return
	}

	// Initialize spawner with both contexts
	spawner, err := usecase.NewUseCaseSuite(essentialCtx, nonEssentialCtx, metricsServer, finalUserConfig, connectionManager)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize use case, exiting")
		return
	}

	log.Info().Msg("Starting spawning processes...")
	go func() {
		err := spawner.Spawn()
		if err != nil {
			log.Error().Err(err).Msg("Failed to spawn processes, exiting")
			rootCancel()
		}
	}()

	<-essentialCtx.Done()

	metricsServer.IncrementMetricsCounter(metrics.ApplicationFinishedCount, wallet.Address, 0)
	// shutdown metrics server
	log.Info().Msg("Shutting down metrics server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error shutting down metrics server")
	}

	log.Info().Msg("Stopping...")

}
