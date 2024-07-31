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
	println("Running worker process for topic", worker.TopicId)

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
			log.Debug().Msgf("Worker anticipated window for topic open nonce %d: %v", worker.TopicId, window)
			mustRecalcWindow = false
		}

		if window.BlockIsWithinWindow(currentBlock) {
			attemptCommit := true

			latestOpenWorkerNonce, err := suite.Node.GetLatestOpenWorkerNonceByTopicId(worker.TopicId)
			if latestOpenWorkerNonce.BlockHeight == 0 || err != nil {
				log.Error().Err(err).Uint64("topicId", worker.TopicId).Msg("Error getting latest open worker nonce on topic")
				attemptCommit = false // Wait some time and try again if block still within the anticipated window
			}

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

		// get the topic data fresh
		topic, err := suite.Node.GetTopicById(reputer.TopicId)
		if err != nil {
			log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Failed to get topic")
			return
		}

		currentBlock, err := suite.Node.GetCurrentChainBlockHeight()
		if err != nil {
			log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Error getting chain block height for reputer job on topic")
			return
		}

		// Try to get the open nonce for the reputer
		newOpenReputerNonce, err := suite.Node.GetLatestOpenReputerNonceByTopicId(reputer.TopicId)
		if newOpenReputerNonce == 0 || err != nil {
			log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("Stopped waiting for open reputer nonce")
			window.WaitForNextReputerAnticipatedWindowToStart(topic, topic.EpochLastEnded+topic.EpochLength, currentBlock)
			continue
		}
		// Cover against repeated open nonce submission
		if newOpenReputerNonce == latestOpenReputerNonce {
			log.Debug().Int64("currentBlock", currentBlock).Int64("lastOpenReputerNonce", latestOpenReputerNonce).Msg("Reputer submission for nonce already done")
			window.WaitForNextReputerAnticipatedWindowToStart(topic, topic.EpochLastEnded+topic.EpochLength, currentBlock)
			continue
		}

		// If nonce ready for reputation, do it
		if newOpenReputerNonce > topic.EpochLastEnded+topic.GroundTruthLag && newOpenReputerNonce <= topic.EpochLastEnded+topic.GroundTruthLag+topic.EpochLength {
			success, err := suite.BuildCommitReputerPayload(reputer, newOpenReputerNonce)
			if err != nil {
				log.Error().Err(err).Uint64("topicId", reputer.TopicId).Msg("Error building and committing worker payload for topic")
			}
			if success {
				log.Info().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Msg("Reputer nonce successfully committed for reputation")
			} else {
				log.Error().Err(err).Uint64("topicId", reputer.TopicId).
					Int64("currentBlock", currentBlock).
					Int64("reputerNonce", newOpenReputerNonce).
					Msg("Error building and committing worker payload")
			}
			latestOpenReputerNonce = newOpenReputerNonce
		} else {
			log.Info().Int64("currentBlock", currentBlock).Int64("reputerNonce", newOpenReputerNonce).Msg("Reputer nonce not ready for reputation")
		}
		window.WaitForNextReputerAnticipatedWindowToStart(topic, topic.EpochLastEnded+topic.EpochLength, currentBlock)
	}
}
