package usecase

import (
	"log"
	"sync"
	"time"

	"allora_offchain_node/repo/query"
	"allora_offchain_node/types"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type ProcessSpawner struct {
	Config types.Config
}

func NewProcessSpawner(config types.Config) ProcessSpawner {
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
			s.runWorkerProcess(workerConfig)
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

func (s *ProcessSpawner) runWorkerProcess(workerConfig types.WorkerConfig) {
	topic, err := query.GetTopicById(workerConfig.TopicId)
	if err != nil {
		log.Println("Quitting because failed to get topic", workerConfig.TopicId)
	}

	// TODO determine approrpiate time to send data per the topic state
	// Should loop until a nonce is found, then only loop every EpochLength

	// Repeat until user interrupts
	for {
		nonce, err := PollOpenWorkerNonce(s.Config.Wallet, workerConfig)
		if err != nil {
			log.Println("Failed to poll open worker nonce", err, "Looping again...")
			continue
		}

		BuildCommitWorkerPayload()
	}
}

func (s *ProcessSpawner) runReputerProcess(reputerConfig types.ReputerConfig) {
	topic, err := query.GetTopicById(reputerConfig.TopicId)
	if err != nil {
		log.Println("Quitting because failed to get topic", reputerConfig.TopicId)
	}

	// TODO idempotently stake...
	GetReputerStakeInTopic(reputerConfig.TopicId, ADDR)

	// TODO determine approrpiate time to send data per the topic state
	// Should loop until a nonce is found, then only loop every EpochLength

	// Repeat until user interrupts
	for {
		nonce, err := PollOpenReputerNonce(s.Config.Options, reputerConfig)
		if err != nil {
			log.Println("Failed to poll open worker nonce: ", err, "Looping again...")
			continue
		}

		BuildCommitReputerPayload()
	}
}

// TODO look at allora-inference-base to see how keys are stored
var ADDR = "allo123"
