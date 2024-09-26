package lib

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
)

// SendDataWithRetry attempts to send data with a uniform backoff strategy for retries.
// uniform backoff is preferred to avoid exiting the open submission windows
func (node *NodeConfig) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, successMsg string) (*cosmosclient.Response, error) {
	var txResp *cosmosclient.Response
	var err error
	for retryCount := int64(0); retryCount <= node.Wallet.MaxRetries; retryCount++ {
		txResponse, err := node.Chain.Client.BroadcastTx(ctx, node.Chain.Account, req)
		txResp = &txResponse
		if err == nil {
			log.Debug().Str("msg", successMsg).Str("txHash", txResp.TxHash).Msg("Success")
			return txResp, nil
		}
		if strings.Contains(err.Error(), "cannot update EMA") {
			log.Error().Err(err).Str("msg", successMsg).Msg("Already sent data for this epoch, no retry")
			return nil, err
		}
		// Log the error for each retry.
		log.Error().Err(err).Str("msg", successMsg).Msgf("Failed, retrying... (Retry %d/%d)", retryCount, node.Wallet.MaxRetries)
		// Wait for the uniform delay before retrying
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(time.Duration(node.Wallet.Delay) * time.Second):
		}
	}
	// All retries failed, return the last error
	return nil, err
}
