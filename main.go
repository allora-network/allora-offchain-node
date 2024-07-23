package main

import (
	"log"
	"sync"

	reputerCoinGecko "allora_offchain_node/pkg/reputer_coingecko_l1_norm"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
	worker20min "allora_offchain_node/pkg/worker_coin_predictor_20min"
)

var TheConfig = Config{
	Options: ConfigOptions{
		Wallet:           "keys.txt",
		RequestRetries:   3,
		Node:             "http://rpc.allora.network",
		LoopSeconds:      60,
		MinStakeToRepute: "100",
	},
	Worker: []WorkerConfig{
		{
			TopicId:             "1",
			InferenceEntrypoint: worker10min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  nil,
		},
		{
			TopicId:             "2",
			InferenceEntrypoint: worker20min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  worker20min.NewAlloraEntrypoint(),
		},
	},
	Reputer: []ReputerConfig{
		{
			TopicId:           "1",
			ReputerEntrypoint: reputerCoinGecko.NewAlloraEntrypoint(),
		},
	},
}

// Check that each assigned entrypoint in `TheConfig` actually can be used
// for the intended purpose, else throw error
func validateConfigEntrypoints(config Config) {
	for _, workerConfig := range config.Worker {
		if workerConfig.InferenceEntrypoint != nil && !workerConfig.InferenceEntrypoint.CanInfer() {
			log.Fatal("Invalid inference entrypoint: ", workerConfig.InferenceEntrypoint)
		}
		if workerConfig.ForecastEntrypoint != nil && !workerConfig.ForecastEntrypoint.CanForecast() {
			log.Fatal("Invalid forecast entrypoint: ", workerConfig.ForecastEntrypoint)
		}
	}

	for _, reputerConfig := range config.Reputer {
		if reputerConfig.ReputerEntrypoint != nil && !reputerConfig.ReputerEntrypoint.CanCalcLoss() {
			log.Fatal("Invalid loss entrypoint: ", reputerConfig.ReputerEntrypoint)
		}
	}
}

func main() {
	validateConfigEntrypoints(TheConfig)
	var wg sync.WaitGroup

	// Run worker processes
	alreadyStartedWorkerForTopic := make(map[string]bool)
	for _, worker := range TheConfig.Worker {
		if _, ok := alreadyStartedWorkerForTopic[worker.TopicId]; ok {
			log.Println("Worker already started for topicId: ", worker.TopicId)
			continue
		}
		alreadyStartedWorkerForTopic[worker.TopicId] = true

		wg.Add(1)
		go func(worker WorkerConfig) {
			defer wg.Done()
			runWorkerProcess(worker.TopicId, worker.InferenceEntrypoint, worker.ForecastEntrypoint)
		}(worker)
	}

	// Run reputer processes
	alreadyStartedReputerForTopic := make(map[string]bool)
	for _, reputer := range TheConfig.Reputer {
		if _, ok := alreadyStartedReputerForTopic[reputer.TopicId]; ok {
			log.Println("Reputer already started for topicId: ", reputer.TopicId)
			continue
		}
		alreadyStartedReputerForTopic[reputer.TopicId] = true

		wg.Add(1)
		go func(reputer ReputerConfig) {
			defer wg.Done()
			runReputerProcess(reputer.TopicId, reputer.ReputerEntrypoint)
		}(reputer)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func runWorkerProcess(topicId string, inferenceEntrypoint, forecastEntrypoint AlloraEntrypoint) {

	// TODO get topic data then loop through epochs...

	if inferenceEntrypoint != nil && inferenceEntrypoint.CanInfer() {
		log.Println("Inference entrypoint: ", inferenceEntrypoint.Name(), " for topicId: ", topicId)
	} else {
		log.Println("No inference entrypoint for topicId: ", topicId)
	}

	if forecastEntrypoint != nil && forecastEntrypoint.CanForecast() {
		log.Println("Forecast entrypoint: ", forecastEntrypoint.Name(), " for topicId: ", topicId)
	} else {
		log.Println("No forecast entrypoint for topicId: ", topicId)
	}
}

func runReputerProcess(topicId string, lossEntrypoint AlloraEntrypoint) {

	// TODO get topic data then loop through epochs...

	// TODO idempotently stake...

	if lossEntrypoint != nil && lossEntrypoint.CanCalcLoss() {
		log.Println("Loss entrypoint: ", lossEntrypoint.Name(), " for topicId: ", topicId)
	} else {
		log.Println("No loss entrypoint for topicId: ", topicId)
	}
}
