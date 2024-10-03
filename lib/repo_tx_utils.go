package lib

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
)

const ERROR_MESSAGE_EMA_ALREADY_SENT = "cannot update EMA"
const ERROR_MESSAGE_TX_INCLUDED_IN_BLOCK = "waiting for next block"

// SendDataWithRetry attempts to send data with a uniform backoff strategy for retries.
// uniform backoff is preferred to avoid exiting the open submission windows
func (node *NodeConfig) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, successMsg string) (*cosmosclient.Response, error) {
	var txResp *cosmosclient.Response
	var err error
	var hadEOFTxError bool
	for retryCount := int64(0); retryCount <= node.Wallet.MaxRetries; retryCount++ {
		txResponse, err := node.Chain.Client.BroadcastTx(ctx, node.Chain.Account, req)
		txResp = &txResponse
		if err == nil {
			log.Debug().Str("msg", successMsg).Str("txHash", txResp.TxHash).Msg("Success")
			return txResp, nil
		}
		if strings.Contains(err.Error(), ERROR_MESSAGE_TX_INCLUDED_IN_BLOCK) {
			if !hadEOFTxError {
				hadEOFTxError = true
				log.Warn().Err(err).Str("msg", successMsg).Msg("Tx sent, waiting for tx to be included in a block, retry")
			}
		}
		if strings.Contains(err.Error(), ERROR_MESSAGE_EMA_ALREADY_SENT) {
			if hadEOFTxError {
				log.Info().Str("msg", successMsg).Msg("Confirmed previously sent tx accepted")
				return nil, err
			} else {
				log.Info().Err(err).Str("msg", successMsg).Msg("Already sent data for this epoch, no retry")
				return nil, err
			}
		}
		// Log the error for each retry.
		log.Error().Err(err).Str("msg", successMsg).Msgf("Failed, retrying... (Retry %d/%d)", retryCount, node.Wallet.MaxRetries)
		// Wait for the uniform delay before retrying
		time.Sleep(time.Duration(node.Wallet.Delay) * time.Second)
	}
	// All retries failed, return the last error
	return nil, err
}
