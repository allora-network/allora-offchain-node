package usecase

import (
	"log"
	"time"
	"io"
	"net/http"
    "encoding/json"


	"allora_offchain_node/types"
	"allora_offchain_node/repo/query"
	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

func BuildCommitWorkerPayload() (emissions.WorkerDataBundle, bool, error) {
	// TODO
	// 1. Compute inferences
	// 2. Compute forecasts
	// 3. Sign, organize into bundle, and commit bundle to chain using retries
	successfulCommit := true
	return emissions.WorkerDataBundle{}, successfulCommit, nil
}

func calculateNextNonceCheckTime(topic emissions.Topic, noncePercentEarlyArrival int64) time.Duration {
    nextNonceTimeInSeconds := (topic.EpochLastEnded + topic.EpochLength) * 5 // assuming each block is 5 seconds
    earlyArrivalNonceTimeInSeconds := nextNonceTimeInSeconds - (nextNonceTimeInSeconds * noncePercentEarlyArrival / 100)
    return time.Duration(earlyArrivalNonceTimeInSeconds) * time.Second
}

func spamCheckTopic(workerConfig types.WorkerConfig, oldTopic emissions.Topic) {
    spamTicker := time.NewTicker(3 * time.Second) // check every 3 seconds
    defer spamTicker.Stop()
	var newTopic emissions.Topic
	var err error

    for range spamTicker.C {
        newTopic, err = query.GetTopicById(workerConfig.TopicId)
        if err != nil {
            log.Println("Unable to get Topic in Spamcheck range", workerConfig.TopicId)
            continue
        }

        if newTopic.EpochLastEnded != oldTopic.EpochLastEnded {
            log.Println("New nonce found, will start the process again")
            BuildCommitWorkerPayload()
            break
        }
    }
}

func GetCurrentChainBlockHeight(rpcURL string) (int64, error) {
	req, err := http.NewRequest("GET", rpcURL+"/block", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Unmarshal the JSON response
	var result struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Block struct {
				Header struct {
					Height int64 `json:"height"`
				} `json:"header"`
			} `json:"block"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, err
	}

	return result.Result.Block.Header.Height, nil
}
