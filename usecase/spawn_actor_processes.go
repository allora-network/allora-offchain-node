package usecase

import (
	"allora_offchain_node/lib"
	"log"
	"sync"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func (suite *UseCaseSuite) Spawn() {
	var wg sync.WaitGroup

	// Run worker process per topic
	alreadyStartedWorkerForTopic := make(map[emissions.TopicId]bool)
	for _, worker := range suite.Node.Worker {
		if _, ok := alreadyStartedWorkerForTopic[worker.TopicId]; ok {
			log.Println("Worker already started for topicId: ", worker.TopicId)
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
			log.Println("Reputer already started for topicId: ", reputer.TopicId)
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
	println("Running worker process for topic", worker.TopicId)

	topic, err := suite.Node.GetTopicById(worker.TopicId)
	if err != nil {
		log.Println("Failed to get topic", worker.TopicId, "Does it exist?")
		return
	}

	registered := suite.Node.RegisterWorkerIdempotently(worker)
	if !registered {
		log.Println("Failed to register worker for topic", worker.TopicId)
		return
	}

	mustRecalcWindow := true
	window := AnticipatedWindow{}
	for {
		currentBlock, err := suite.Node.GetCurrentChainBlockHeight()
		if err != nil {
			log.Println("Error getting chain block height for worker job on topic", worker.TopicId, err)
			return
		}

		if mustRecalcWindow {
			window = suite.CalcSoonestAnticipatedWindow(topic, currentBlock)
			mustRecalcWindow = false
		}

		if window.BlockIsWithinWindow(currentBlock) {
			attemptCommit := true

			latestOpenWorkerNonce, err := suite.Node.GetLatestOpenWorkerNonceByTopicId(worker.TopicId)
			if err != nil {
				log.Println("Error getting latest open worker nonce on topic", worker.TopicId, err)
				attemptCommit = false // Wait some time and try again if block still within the anticipated window
			}

			if attemptCommit {
				success, err := suite.BuildCommitWorkerPayload(latestOpenWorkerNonce)
				if err != nil {
					log.Println("Error building and committing worker payload for topic", worker.TopicId, err)
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
	println("Running reputer process for topic", reputer.TopicId)

	topic, err := suite.Node.GetTopicById(reputer.TopicId)
	if err != nil {
		log.Println("Failed to get topic", reputer.TopicId, "Does it exist?")
		return
	}

	registeredAndStaked := suite.Node.RegisterAndStakeReputerIdempotently(reputer)
	if !registeredAndStaked {
		log.Println("Failed to register or sufficiently stake reputer for topic", reputer.TopicId)
		return
	}

	mustRecalcWindow := true
	window := AnticipatedWindow{}
	for {
		currentBlock, err := suite.Node.GetCurrentChainBlockHeight()
		if err != nil {
			log.Println("Error getting chain block height for reputer job on topic", reputer.TopicId, err)
			return
		}

		if mustRecalcWindow {
			window = suite.CalcSoonestAnticipatedWindow(topic, currentBlock)
			mustRecalcWindow = false
		}

		if window.BlockIsWithinWindow(currentBlock) {
			attemptCommit := true

			latestOpenWorkerNonce, err := suite.Node.GetLatestOpenReputerNonceByTopicId(reputer.TopicId)
			if err != nil {
				log.Println("Error getting latest open reputer nonce on topic", reputer.TopicId, err)
				attemptCommit = false // Wait some time and try again if block still within the anticipated window
			}

			if attemptCommit {
				success, err := suite.BuildCommitReputerPayload(latestOpenWorkerNonce)
				if err != nil {
					log.Println("Error building and committing reputer payload for topic", reputer.TopicId, err)
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
