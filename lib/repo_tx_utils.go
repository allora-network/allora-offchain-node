package lib

import (
	"allora_offchain_node/transaction"
	"allora_offchain_node/types"
	"context"

	errorsmod "cosmossdk.io/errors"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

func (rpcManager *RPCManager) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, infoMsg string, timeoutHeight uint64) (*coretypes.ResultBroadcastTx, error) {
	// Excess fees correction factor translated to fees using configured gas prices
	// This value is updated by the fee price update routine - making copy for consistency within method
	gasPrice := GetGasPrice()
	walletConfig, err := rpcManager.GetWalletConfig()
	if err != nil {
		return nil, err
	}
	wallet, err := rpcManager.GetWallet()
	if err != nil {
		return nil, err
	}
	txParams := &types.TransactionParams{
		ChainID:       walletConfig.ChainId,
		Denom:         DEFAULT_BOND_DENOM,
		Prefix:        ADDRESS_PREFIX,
		Sequence:      wallet.GetSequence(),
		AccNum:        wallet.AccountNumber,
		PrivKey:       wallet.PrivKey,
		PubKey:        wallet.PubKey,
		TimeoutHeight: timeoutHeight,
		GasEstimationConfig: types.GasEstimationConfig{
			BaseGas:     200000,
			GasPerByte:  1,
			MinGasPrice: gasPrice,
		},
	}

	txNode := rpcManager.GetCurrentTxNode()

	for retryCount := int64(0); retryCount <= walletConfig.MaxRetries; retryCount++ {
		log.Debug().Msgf("SendDataWithRetry iteration started (%d/%d)", retryCount, walletConfig.MaxRetries)

		// Create tx without fees to simulate tx creation and get estimated gas and seq number
		txResp, _, err := transaction.SendTransactionViaRPC(ctx, txNode.ServerAddress, txParams, wallet.GetSequence(), false, req)
		if err == nil {
			if txResp != nil {
				log.Printf("Transaction sent successfully: %v\n", txResp.Hash.String())
			} else {
				log.Error().Msg("Transaction sent successfully but response is nil")
			}
			// TODO lock this in the overall wallet, not just the chain object inside each node
			wallet.SetSequence(wallet.GetSequence() + 1)
			return txResp, nil
		}

		// Handle error on broadcasting
		errorResponse, err := ProcessErrorTx(ctx, err, infoMsg, retryCount, walletConfig.MaxRetries, txNode)
		switch errorResponse {
		case ErrorProcessingOk:
			return txResp, nil
		case ErrorProcessingError:
			// Error has not been handled, sleep and retry with regular delay
			if err != nil {
				log.Error().Err(err).Str("rpc", txNode.ServerAddress).Str("msg", infoMsg).Msgf("Failed, retrying... (Retry %d/%d)", retryCount, walletConfig.MaxRetries)
				// Wait for the uniform delay before retrying
				if DoneOrWait(ctx, walletConfig.RetryDelay) {
					return nil, ctx.Err()
				}
				continue
			}
		case ErrorProcessingContinue:
			// Error has not been handled, just continue next iteration
			continue
		case ErrorProcessingFees:
			// Error has not been handled, just mark as recalculate fees on this iteration
			log.Info().Msg("Insufficient fees, marking fee recalculation on tx broadcasting for retrial")
			continue
		case ErrorProcessingFailure:
			return nil, errorsmod.Wrapf(err, "tx failed and not retried")
		case ErrorProcessingSwitchingNode:
			return nil, err
		default:
			return nil, errorsmod.Wrapf(err, "failed to process error")
		}
	}

	return nil, errorsmod.Wrapf(ErrUnexpectedError, "Tx failed after max retries")
}
