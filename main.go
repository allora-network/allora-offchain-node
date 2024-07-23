package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"

	reputer "allora_offchain_node/pkg/reputer_coingecko_l1_norm"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
	worker20min "allora_offchain_node/pkg/worker_coin_predictor_20min"
)

var AvailableEntrypoints = map[string]AlloraEntrypoint{
	worker10min.AlloraEntrypoint{}.Name(): worker10min.NewAlloraEntrypoint(),
	worker20min.AlloraEntrypoint{}.Name(): worker20min.NewAlloraEntrypoint(),
	reputer.AlloraEntrypoint{}.Name():     reputer.NewAlloraEntrypoint(),
}

type TopicId = string
type TopicToEntrypoint map[TopicId]*AlloraEntrypoint

// Ensure that entrypoints in this file are same as in JSON config file
func matchTopicToEntrypoints(config Config) (
	map[string]bool,
	TopicToEntrypoint,
	TopicToEntrypoint,
	TopicToEntrypoint,
) {
	uniqueWorkerTopicIds := make(map[TopicId]bool)
	inferenceEntrypoints := make(TopicToEntrypoint)
	forecastEntrypoints := make(TopicToEntrypoint)
	reputerEntrypoints := make(TopicToEntrypoint)

	for _, workerConfig := range config.Worker {
		if _, ok := uniqueWorkerTopicIds[workerConfig.TopicId]; !ok {
			uniqueWorkerTopicIds[workerConfig.TopicId] = true
		}
		log.Print("inferenceConfig: ", workerConfig.InferenceEntrypoint)
		if val, ok := AvailableEntrypoints[workerConfig.InferenceEntrypoint]; ok && val.CanInfer() {
			inferenceEntrypoints[workerConfig.TopicId] = &val
		}
		log.Print("forecastConfig: ", workerConfig.ForecastEntrypoint)
		if val, ok := AvailableEntrypoints[workerConfig.ForecastEntrypoint]; ok && val.CanForecast() {
			forecastEntrypoints[workerConfig.ForecastEntrypoint] = &val
		}
	}

	for _, reputerConfig := range config.Reputer {
		log.Print("reputerConfig: ", reputerConfig.ReputerEntrypoint)
		log.Print("AvailableEntrypoints", AvailableEntrypoints)
		log.Print(AvailableEntrypoints[reputerConfig.ReputerEntrypoint].CanCalcLoss())
		if val, ok := AvailableEntrypoints[reputerConfig.ReputerEntrypoint]; ok && val.CanCalcLoss() {
			reputerEntrypoints[reputerConfig.ReputerEntrypoint] = &val
		}
	}

	return uniqueWorkerTopicIds, inferenceEntrypoints, forecastEntrypoints, reputerEntrypoints
}

func main() {
	// Read config.json file
	configFile, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()

	byteValue, _ := io.ReadAll(configFile)

	var config Config
	json.Unmarshal(byteValue, &config)

	// Organize which entrypoints user wants to run per topic
	uniqueWorkerTopicIds,
		inferenceEntrypoints,
		forecastEntrypoints,
		reputerEntrypoints := matchTopicToEntrypoints(config)

	log.Print(config)
	log.Print("uniqueWorkerTopicIds: ", uniqueWorkerTopicIds)
	log.Print("inferenceEntrypoints: ", inferenceEntrypoints)
	log.Print("forecastEntrypoints: ", forecastEntrypoints)
	log.Print("reputerEntrypoints: ", reputerEntrypoints)

	// Run worker and reputer processes
	var wg sync.WaitGroup
	for topicId := range uniqueWorkerTopicIds {
		wg.Add(1)
		go func(topicId string, inferenceEntrypoint, forecastEntrypoint *AlloraEntrypoint) {
			defer wg.Done()
			runWorkerProcess(topicId, *inferenceEntrypoint, *forecastEntrypoint)
		}(topicId, inferenceEntrypoints[topicId], forecastEntrypoints[topicId])
	}

	for topicId, reputerEntrypoint := range reputerEntrypoints {
		wg.Add(1)
		go func(topicId string, reputerEntrypoint *AlloraEntrypoint) {
			defer wg.Done()
			runReputerProcess(topicId, *reputerEntrypoint)
		}(topicId, reputerEntrypoint)
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
