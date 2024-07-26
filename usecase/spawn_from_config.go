package usecase

import (
	"log"
	"sync"
	"time"

	"allora_offchain_node/repo/query"
	"allora_offchain_node/repo/client"
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type ProcessSpawner struct {
	Config types.UserConfig
}

func NewProcessSpawner(config types.UserConfig) ProcessSpawner {
	config.ValidateConfigEntrypoints()
	return ProcessSpawner{Config: config}
}

func (s *ProcessSpawner) Spawn() {
	var wg sync.WaitGroup

	// Run worker processes
	alreadyStartedWorkerForTopic := make(map[emissions.TopicId]bool)
	for _, worker := range s.Config.Worker {
		if _, ok := alreadyStartedWorkerForTopic[worker.TopicId]; ok {
			log.Println("Worker already started for topicId: ", worker.TopicId)
			continue
		}
		alreadyStartedWorkerForTopic[worker.TopicId] = true

		wg.Add(1)
		go func(workerConfig types.WorkerConfig) {
			defer wg.Done()

			config := client.IndividualUserConfig {
				Wallet: s.Config.Wallet,
				Worker: &workerConfig,
			}
			node, err := config.NewAlloraChain()
			if err != nil {
				log.Println("Failed to initialize allora client", err)
			}
			s.runWorkerProcess(node)
		}(worker)
	}

	// Run reputer processes
	alreadyStartedReputerForTopic := make(map[emissions.TopicId]bool)
	for _, reputer := range s.Config.Reputer {
		if _, ok := alreadyStartedReputerForTopic[reputer.TopicId]; ok {
			log.Println("Reputer already started for topicId: ", reputer.TopicId)
			continue
		}
		alreadyStartedReputerForTopic[reputer.TopicId] = true

		wg.Add(1)
		go func(reputerConfig types.ReputerConfig) {
			defer wg.Done()
			s.runReputerProcess(reputerConfig)
		}(reputer)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func (s *ProcessSpawner) waitForNextLoop() {
	time.Sleep(time.Duration(s.Config.Wallet.LoopSeconds) * time.Second)
}

func (s *ProcessSpawner) runWorkerProcess(node *client.NodeConfig) {
	// 1. Retrieve topic object by ID

	// 2. Calculate soonest start and end times for current block window
	// 3. Retrieve current chain block height
	// 4. If retrieval fails, log error and continue

	// 5. If current block height is within soonest start and end times:
	// a. Perform action (BuildCommitWorkerPayload)

	// 6. Calculate next nonce check time (early arrival based on a percentage)
	// 7. Set up ticker to trigger at early arrival nonce check time

	// 8. While ticker is running:
	// a. Perform spam check:
	// 	i. Set up spam ticker to trigger every 3 seconds
	// 	ii. While spam ticker is running:
	// 		1. Retrieve topic object again
	// 		2. If retrieval fails, log error and continue
	// 		3. If topic object's EpochLastEnded field has changed:
	// 			a. Perform action (BuildCommitWorkerPayload)
	// 			b. Break out of spam ticker loop


    topic, err := query.GetTopicById(node.Worker.TopicId)
    if err != nil {
        log.Println("Failed to get topic", node.Worker.TopicId)
        return
    }

    const (
        NoncePercentEarlyArrival = 50
        WindowLength             = 2 // remove this when WindowLength added to the topic query response
    )

    soonestStart := topic.EpochLastEnded
    soonestEnd := soonestStart + WindowLength

    firstCurrentBlock, err := GetCurrentChainBlockHeight(s.Config.Wallet.NodeRpc)
    if err != nil {
        log.Println("Error getting chain block height", err)
    } else if firstCurrentBlock >= soonestStart && firstCurrentBlock <= soonestEnd {
        log.Println("First current block is within the window, processing worker payload now")
        BuildCommitWorkerPayload()
    }

    nextNonceCheckTime := calculateNextNonceCheckTime(topic, NoncePercentEarlyArrival)
    ticker := time.NewTicker(nextNonceCheckTime)
    defer ticker.Stop()

    for range ticker.C {
        log.Println("Early arrival nonce check time reached")
        spamCheckTopic(*node.Worker, topic)
    }
}

func (s *ProcessSpawner) runReputerProcess(reputerConfig types.ReputerConfig) {
	println("Running reputer process")
	// topic, err := query.GetTopicById(reputerConfig.TopicId)
	// if err != nil {
	// 	log.Println("Quitting because failed to get topic", reputerConfig.TopicId)
	// }

	// // TODO idempotently stake...
	// GetReputerStakeInTopic(reputerConfig.TopicId, ADDR)

	// // TODO determine approrpiate time to send data per the topic state
	// // Should loop until a nonce is found, then only loop every EpochLength

	// // Repeat until user interrupts
	// for {
	// 	nonce, err := PollOpenReputerNonce(s.Config.Options, reputerConfig)
	// 	if err != nil {
	// 		log.Println("Failed to poll open worker nonce: ", err, "Looping again...")
	// 		continue
	// 	}

	// 	BuildCommitReputerPayload()
	// }
}

// TODO look at allora-inference-base to see how keys are stored
var ADDR = "allo123"
