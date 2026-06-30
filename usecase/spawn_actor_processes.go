package usecase

import (
	"allora_offchain_node/lib"
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	errorsmod "cosmossdk.io/errors"
	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/rand"
)

// launchGasRoutine initializes gas prices and starts the auto-update routine if needed
func (suite *UseCaseSuite) launchGasRoutine(ctx context.Context, walletConfig *lib.WalletConfig, wallet *lib.Wallet) error {
	// Initialize gas prices explicitly first
	err := suite.UpdateGasPrice(ctx, wallet, walletConfig)
	if err != nil {
		log.Error().Err(err).Msg("Error updating gas prices in auto mode - RPC availability issue?")
		return err
	}
	// After initialization, start auto-update routine
	go suite.UpdateGasPriceRoutine(ctx, wallet, walletConfig)
	return nil
}

// Spawns the actor processes and any associated non-essential routines
func (suite *UseCaseSuite) Start() error {

	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		log.Error().Err(err).Msg("Error getting wallet")
		return err
	}
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		log.Error().Err(err).Msg("Error getting wallet config")
		return err
	}
	if walletConfig.GasPrices == lib.AutoGasPrices {
		if err := suite.launchGasRoutine(suite.nonEssentialCtx, walletConfig, wallet); err != nil {
			return err
		}
	} else {
		price, err := strconv.ParseFloat(walletConfig.GasPrices, 64)
		if err != nil {
			log.Error().Err(err).Msg("Invalid gas prices format")
			return err
		} else {
			log.Debug().Float64("gasPrice", price).Msg("Setting gas prices manually")
			lib.SetGasPrice(price)
		}
	}

	for _, worker := range suite.UserConfig.Worker {
		suite.startWorker(suite.essentialCtx, worker)

		if lib.DoneOrWait(suite.essentialCtx, walletConfig.LaunchRoutineDelay) {
			log.Error().Msg("Worker process finished")
		}
	}
	for _, reputer := range suite.UserConfig.Reputer {
		suite.startReputer(suite.essentialCtx, reputer)

		if lib.DoneOrWait(suite.essentialCtx, walletConfig.LaunchRoutineDelay) {
			log.Error().Msg("Reputer process finished")
		}
	}

	go func() {
		<-suite.essentialCtx.Done()
		log.Info().Msg("Essential ctx done, closing")
		suite.swm.Stop()
	}()

	suite.swm.Join()
	return nil
}

// Attempts to build and commit a worker payload for a given nonce
func (suite *UseCaseSuite) processWorkerPayload(ctx context.Context, worker lib.WorkerConfig, nonce emissionstypes.Nonce, timeoutHeight int64) error {
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet config")
	}
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet")
	}

	// Check whitelist with RPC timeout
	isWhitelisted, err := WithTimeoutResult(ctx, time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
		func(ctx context.Context) (bool, error) {
			node, err := suite.ConnectionManager.GetCurrentQueryNode()
			if err != nil {
				return false, fmt.Errorf("failed to get current query node: %w", err)
			}
			return node.CanSubmitWorker(ctx, worker.TopicId, wallet.Address)
		})

	if err != nil {
		log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Failed to check if worker is whitelisted")
		return err
	}
	if !isWhitelisted {
		log.Error().Uint64("topicId", worker.TopicId).Msg("Worker is not whitelisted in topic, not submitting payload")
		return nil
	}

	// Build and commit payload with transaction timeout
	err = WithTimeout(ctx, time.Duration(walletConfig.TimeoutRPCSecondsTx)*time.Second,
		func(ctx context.Context) error {
			return suite.BuildCommitWorkerPayload(ctx, worker, nonce, uint64(timeoutHeight))
		})

	if err != nil {
		return errorsmod.Wrapf(err, "error building and committing worker payload for topic")
	}

	log.Debug().Uint64("topicId", worker.TopicId).
		Str("actorType", "worker").
		Msg("Successfully finished processing payload")
	return nil
}

func (suite *UseCaseSuite) processReputerPayload(ctx context.Context, reputer lib.ReputerConfig, nonce emissionstypes.Nonce, timeoutHeight int64) error {
	log := log.With().Uint64("topicId", reputer.TopicId).Str("actorType", "reputer").Logger()
	log.Info().Msg("Processing reputer payload")
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet config")
	}
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		return errorsmod.Wrapf(err, "Error getting wallet")
	}

	// Check if reputer can submit
	isWhitelisted, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (bool, error) {
			return WithTimeoutResult(ctx,
				time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
				func(ctx context.Context) (bool, error) {
					return node.CanSubmitReputer(ctx, reputer.TopicId, wallet.Address)
				})
		},
		"check reputer whitelist",
		lib.GRPC_MODE,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check if reputer is whitelisted")
		return err
	}
	if !isWhitelisted {
		log.Error().Msg("Reputer is not whitelisted in topic, not submitting payload")
		return nil
	}

	// Build and commit payload with transaction timeout
	err = WithTimeout(ctx, time.Duration(walletConfig.TimeoutRPCSecondsTx)*time.Second,
		func(ctx context.Context) error {
			return suite.BuildCommitReputerPayload(ctx, reputer, nonce.BlockHeight, uint64(timeoutHeight))
		})

	if err != nil {
		return errorsmod.Wrapf(err, "error building and committing reputer payload for topic")
	}

	log.Debug().Msg("Successfully finished processing payload")
	return nil
}

// Generate jitter between 0 and submissionJitter
func generateRandomJitter(submissionJitter uint64) int64 {
	if submissionJitter == 0 {
		return 0
	}
	source := rand.NewSource(uint64(time.Now().UnixNano())) //nolint:gosec
	rng := rand.New(source)

	maxSafeValue := uint64(math.MaxInt64)
	if submissionJitter > maxSafeValue {
		submissionJitter = maxSafeValue
	}
	return int64(rng.Uint64() % submissionJitter) //nolint:gosec // using a safe max value
}

// Runs the worker process for a given worker config
func (suite *UseCaseSuite) startWorker(ctx context.Context, worker lib.WorkerConfig) {
	// Create a logger with the topicId
	log := log.With().Uint64("topicId", worker.TopicId).Str("actorType", "worker").Logger()

	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet config")
		return
	}
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet")
		return
	}
	// Handle registration
	registered, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (bool, error) {
			return WithTimeoutResult(ctx, time.Duration(walletConfig.TimeoutRPCSecondsRegistration)*time.Second,
				func(ctx context.Context) (bool, error) {
					return suite.RegisterWorkerIdempotently(ctx, worker)
				})
		},
		"RegisterWorkerIdempotently",
		lib.RPC_MODE,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to register for topic, exiting")
		return
	}

	if !registered {
		log.Error().Msg("Failed to register worker for topic, exiting")
		return
	}
	log.Debug().Msg("Worker registered")

	// Using the helper function
	topicInfo, err := queryTopicInfo(ctx, suite, worker)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get topic info for worker")
		return
	}

	// Check if worker is isWhitelisted
	isWhitelisted, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (bool, error) {
			return WithTimeoutResult(ctx, time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
				func(ctx context.Context) (bool, error) {
					return node.CanSubmitWorker(ctx, worker.TopicId, wallet.Address)
				})
		},
		"check worker whitelist",
		lib.GRPC_MODE,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check if worker is whitelisted")
		return
	}
	if !isWhitelisted {
		log.Error().Msg("Worker is not whitelisted in topic, exiting worker process")
		return
	}

	if err := suite.swm.WakeOnWorkerWindowOpen(ctx, worker.TopicId, func(ctx context.Context, nonce emissionstypes.Nonce, height int64) {
		if lib.DoneOrWait(suite.essentialCtx, generateRandomJitter(walletConfig.SubmissionJitter)) {
			return
		}

		if err := suite.processWorkerPayload(ctx, worker, nonce, height+topicInfo.EpochLength); err != nil {
			log.Error().Err(err).Msg("Error processing payload - could not complete transaction")
		}
	}); err != nil {
		log.Error().Err(err).Msg("Could not start worker")
	}
}

// Runs the reputer process for a given reputer config
func (suite *UseCaseSuite) startReputer(ctx context.Context, reputer lib.ReputerConfig) {
	// Create a logger with the topicId
	log := log.With().Uint64("topicId", reputer.TopicId).Str("actorType", "reputer").Logger()
	log.Debug().Msg("Running reputer process for topic")
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet config")
		return
	}
	wallet, err := suite.ConnectionManager.GetWallet()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet")
		return
	}
	// Handle registration and staking
	registeredAndStaked, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (bool, error) {
			return WithTimeoutResult(ctx,
				time.Duration(walletConfig.TimeoutRPCSecondsRegistration)*time.Second,
				func(ctx context.Context) (bool, error) {
					return suite.RegisterAndStakeReputerIdempotently(ctx, reputer)
				})
		},
		"RegisterAndStakeReputerIdempotently",
		lib.RPC_MODE,
	)
	if err != nil {
		log.Error().Err(err).Msg("Error: Failed to register or sufficiently stake for topic")
		return
	}
	if !registeredAndStaked {
		log.Error().Msg("Could not register or sufficiently stake for topic")
		return
	}
	log.Debug().Msg("Reputer registered and staked")

	// Using the helper function
	topicInfo, err := queryTopicInfo(ctx, suite, reputer)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get topic info for reputer")
		return
	}

	// Check if reputer is isWhitelisted
	isWhitelisted, err := lib.RunWithNodeRetry(
		ctx,
		suite.ConnectionManager,
		func(node *lib.NodeConfig) (bool, error) {
			return WithTimeoutResult(ctx,
				time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
				func(ctx context.Context) (bool, error) {
					return node.CanSubmitReputer(ctx, reputer.TopicId, wallet.Address)
				})
		},
		"check reputer whitelist",
		lib.GRPC_MODE,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check if reputer is whitelisted")
		return
	}
	if !isWhitelisted {
		log.Error().Msg("Reputer is not whitelisted in topic, exiting reputer process")
		return
	}

	if err := suite.swm.WakeOnReputerWindowOpen(ctx, reputer.TopicId, func(ctx context.Context, nonce emissionstypes.Nonce, height int64) {
		if lib.DoneOrWait(suite.essentialCtx, generateRandomJitter(walletConfig.SubmissionJitter)) {
			return
		}

		if err := suite.processReputerPayload(ctx, reputer, nonce, height+topicInfo.EpochLength); err != nil {
			log.Error().Err(err).Msg("Error processing payload - could not complete transaction")
		}
	}); err != nil {
		log.Error().Err(err).Msg("Could not start reputer")
	}
}

// resolveMultiLabel maps a topic's on-chain output arity to whether its payloads
// are multi-label (vector). Dispatch across the worker inference, reputer ground
// truth and reputer loss paths keys off this single source of truth (the chain's
// authoritative TopicOutputArity) rather than local config presence or runtime
// vector length. An unspecified or invalid arity is an error so the node fails
// loudly instead of guessing.
func resolveMultiLabel(arity emissionstypes.TopicOutputArity) (bool, error) {
	switch arity {
	case emissionstypes.TopicOutputArity_TOPIC_OUTPUT_ARITY_SINGLE:
		return false, nil
	case emissionstypes.TopicOutputArity_TOPIC_OUTPUT_ARITY_MULTI:
		return true, nil
	default:
		return false, fmt.Errorf("topic has unspecified or invalid output arity (%s)", arity)
	}
}

// Queries the topic info for a given actor type and wallet params from suite
// Wrapper over NodeConfig.GetTopicInfo() with generic config type
func queryTopicInfo[T lib.TopicActor](
	ctx context.Context,
	suite *UseCaseSuite,
	config T,
) (*emissionstypes.Topic, error) {
	walletConfig, err := suite.ConnectionManager.GetWalletConfig()
	if err != nil {
		return nil, errorsmod.Wrapf(err, "Error getting wallet config")
	}
	topicInfo, err := WithTimeoutResult(ctx,
		time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
		func(ctx context.Context) (*emissionstypes.Topic, error) {
			node, err := suite.ConnectionManager.GetCurrentQueryNode()
			if err != nil {
				return nil, fmt.Errorf("failed to get current query node: %w", err)
			}
			return node.GetTopicInfo(ctx, config.GetTopicId())
		})
	if err != nil {
		return nil, errorsmod.Wrapf(err, "failed to get topic info")
	}
	return topicInfo, nil
}
