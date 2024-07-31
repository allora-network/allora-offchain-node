package usecase

import (
	"allora_offchain_node/lib"
	"sync"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/rs/zerolog/log"
)

func (suite *UseCaseSuite) Spawn() {
	var wg sync.WaitGroup

	// Run worker process per topic
	alreadyStartedWorkerForTopic := make(map[emissions.TopicId]bool)
	for _, worker := range suite.Node.Worker {
		if _, ok := alreadyStartedWorkerForTopic[worker.TopicId]; ok {
			log.Debug().Uint64("topicId", worker.TopicId).Msg("Worker already started for topicId")
			continue
		}
		alreadyStartedWorkerForTopic[worker.TopicId] = true

		wg.Add(1)
		go func(worker lib.WorkerConfig) {
			defer wg.Done()
			suite.runWorkerProcess(worker)
		}(worker)
	}

	// Run reputer process per topic
	alreadyStartedReputerForTopic := make(map[emissions.TopicId]bool)
	for _, reputer := range suite.Node.Reputer {
		if _, ok := alreadyStartedReputerForTopic[reputer.TopicId]; ok {
			log.Debug().Uint64("topicId", reputer.TopicId).Msg("Reputer already started for topicId")
			continue
		}
		alreadyStartedReputerForTopic[reputer.TopicId] = true

		wg.Add(1)
		go func(reputer lib.ReputerConfig) {
			defer wg.Done()
			suite.runReputerProcess(reputer)
		}(reputer)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func (suite *UseCaseSuite) runWorkerProcess(worker lib.WorkerConfig) {
	log.Info().Uint64("topicId", worker.TopicId).Msg("Running worker process for topic")

	topic, err := suite.Node.GetTopicById(worker.TopicId)
	if err != nil {
		log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Failed to get topic")
		return
	}

	registered := suite.Node.RegisterWorkerIdempotently(worker)
	if !registered {
		log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Failed to register worker for topic")
		return
	}

	mustRecalcWindow := true
	window := AnticipatedWindow{}
	for {
		currentBlock, err := suite.Node.GetCurrentChainBlockHeight()
		if err != nil {
			log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Error getting chain block height for worker job on topic")
			return
		}

		if mustRecalcWindow {

			window = window.CalcWorkerSoonestAnticipatedWindow(suite, topic, currentBlock)
			log.Debug().Msgf("Worker anticipated window for topic %d open nonce. Open: %f Close $f %v", worker.TopicId, window.SoonestTimeForOpenNonceCheck, window.SoonestTimeForEndOfWorkerNonceSubmission)
			mustRecalcWindow = false
		}

		if window.BlockIsWithinWindow(currentBlock) {
			attemptCommit := true

			latestOpenWorkerNonce, err := suite.Node.GetLatestOpenWorkerNonceByTopicId(worker.TopicId)
			if latestOpenWorkerNonce.BlockHeight == 0 || err != nil {
				log.Warn().Err(err).Uint64("topicId", worker.TopicId).Msg("Error getting latest open worker nonce on topic")
				attemptCommit = false
			}
			log.Info().Int64("latestOpenWorkerNonce", latestOpenWorkerNonce.BlockHeight).Uint64("topicId", worker.TopicId).Msg("Got latest open worker nonce")

			if attemptCommit {
				success, err := suite.BuildCommitWorkerPayload(worker, latestOpenWorkerNonce)
				if err != nil {
					log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Error building and committing worker payload for topic")
				}
				if success {
					mustRecalcWindow = true
					window.WaitForNextAnticipatedWindowToStart(currentBlock, topic.EpochLength)
					continue
				}
			}

			suite.WaitWithinAnticipatedWindow()
		} else {
			log.Debug().Msgf("Block %d is not within window. Open: %f Close: %f", currentBlock, window.SoonestTimeForOpenNonceCheck, window.SoonestTimeForEndOfWorkerNonceSubmission)
			window.WaitForNextAnticipatedWindowToStart(currentBlock, topic.EpochLength)
		}
	}
}

func (suite *UseCaseSuite) runReputerProcess(reputer lib.ReputerConfig) {
	log.Debug().Uint64("topicId", reputer.TopicId).Msg("Running reputer process for topic")

	registeredAndStaked := suite.Node.RegisterAndStakeReputerIdempotently(reputer)
	if !registeredAndStaked {
		log.Error().Uint64("topicId", reputer.TopicId).Msg("Failed to register or sufficiently stake reputer for topic")
		return
	}

	var latestOpenReputerNonce lib.BlockHeight
	window := AnticipatedWindow{}

	for {
		// if we reach the epochLength, we check for open reputer nonces
		// - if they are eligible for reputation (gt_lag has passed) then we submit the reputer nonce
		// - if they are not eligible for reputation, we wait until next epochLength
		//
		// if we haven't reached the epochLength, we wait until we reach it

		// TODO Make an abstraction where this is done inside the creation of worker
		topic, err := suite.Node.GetTopicById(reputer.TopicId)
		if err != nil {
			log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to get topic")
			return
		}
		nextEpochStartsAt := topic.EpochLastEnded + topic.EpochLength

		currentBlock, err := suite.Node.GetCurrentChainBlockHeight()
		if err != nil {
			log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Error getting chain block height for reputer job on topic")
			return
		}
		log.Trace().Int64("nextEpochLength", currentBlock).Int64("currentBlock", currentBlock).Msg("New Reputer iteration")
		// Try to get the open nonce for the reputer
		newOpenReputerNonces, err := suite.Node.GetOpenReputerNonces(reputer.TopicId)
		if err != nil {
			log.Warn().Err(err).Uint64("topicId", reputer.TopicId).Msg("Error getting open reputer nonces on topic")
			window.WaitForNextWindowToStart(currentBlock, nextEpochStartsAt)
			continue
		}
		if len(newOpenReputerNonces.Nonces) == 0 {
			log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("No open reputer nonce found")
			window.WaitForNextWindowToStart(currentBlock, nextEpochStartsAt)
			continue
		} else {
			log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("Open reputer nonce found")
		}

		// Process the open nonces
		for _, reputerNonce := range newOpenReputerNonces.Nonces {
			newOpenReputerNonce := reputerNonce.ReputerNonce.BlockHeight
			if newOpenReputerNonce == 0 || err != nil {
				log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("Stopped waiting for open reputer nonce")
				continue
			}
			// Cover against repeated open nonce submission
			if newOpenReputerNonce == latestOpenReputerNonce {
				log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("Reputer submission for nonce already done")
				continue
			}

			// If nonce ready for reputation, do it
			low_window := reputerNonce.ReputerNonce.BlockHeight + topic.GroundTruthLag
			high_window := reputerNonce.ReputerNonce.BlockHeight + topic.GroundTruthLag + topic.EpochLength
			log.Trace().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Int64("low_window", low_window).Int64("high_window", high_window).Msg("Checking reputer nonce for reputation")
			if currentBlock >= low_window && currentBlock <= high_window {
				log.Debug().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Int64("low_window", low_window).Int64("high_window", high_window).Msg("Processing reputer nonce for reputation")
				success, err := suite.BuildCommitReputerPayload(reputer, newOpenReputerNonce)
				if err != nil {
					log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Error building and committing reputer payload for topic")
				}
				if success {
					log.Info().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Msg("Reputer nonce successfully committed for reputation")
				} else {
					log.Error().Err(err).Uint64("topicId", reputer.TopicId).
						Int64("currentBlock", currentBlock).
						Int64("reputerNonce", newOpenReputerNonce).
						Msg("Error building and committing reputer payload, exhausted retries")
				}
				latestOpenReputerNonce = newOpenReputerNonce
			} else {
				log.Trace().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Msg("Reputer nonce not ready for reputation")
			}
		}
		// Finally, whatever the result, if reached here, wait for the next epoch
		window.WaitForNextWindowToStart(currentBlock, nextEpochStartsAt)
	} // efor infinite loop
}
