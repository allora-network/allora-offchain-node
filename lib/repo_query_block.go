package lib

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

func (node *NodeConfig) GetCurrentChainBlockHeight() (int64, error) {
	req, err := http.NewRequest("GET", node.Wallet.NodeRpc+"/block", nil)
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
					Height string `json:"height"`
				} `json:"header"`
			} `json:"block"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, err
	}

	height, err := strconv.ParseInt(result.Result.Block.Header.Height, 10, 64)
	if err != nil {
		return 0, err
	}
	log.Debug().Int64("height", height).Msg("Current block height")
	return height, nil
}
