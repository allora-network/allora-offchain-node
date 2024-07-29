package lib

import (
	"context"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
)

func (node *NodeConfig) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, successMsg string) (*cosmosclient.Response, error) {
	var txResp *cosmosclient.Response
	var err error
	for retryCount := int64(0); retryCount <= node.Wallet.MaxRetries; retryCount++ {
		txResponse, err := node.Chain.Client.BroadcastTx(ctx, node.Chain.Account, req)
		txResp = &txResponse
		if err == nil {
			log.Debug().Str("msg", successMsg).Str("txHash", txResp.TxHash).Msg("Success")
			break
		}
		// Log the error for each retry.
		log.Error().Str("msg", successMsg).Msgf("Failed, retrying... (Retry %d/%d)", retryCount, node.Wallet.MaxRetries)
		// Generate a random number between MinDelay and MaxDelay
		randomDelay := rand.Intn(int(node.Wallet.MaxDelay-node.Wallet.MinDelay+1)) + int(node.Wallet.MinDelay)
		// Apply exponential backoff to the random delay
		backoffDelay := randomDelay << retryCount
		// Wait for the calculated delay before retrying
		time.Sleep(time.Duration(backoffDelay) * time.Second)
	}
	return txResp, err
}
