package lib

import (
	"allora_offchain_node/lib/transaction"
	types "allora_offchain_node/lib/types"
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

func (connectionManager *ConnectionManager) SendDataWithRetry(ctx context.Context, req sdktypes.Msg, infoMsg string, timeoutHeight uint64) (*coretypes.ResultBroadcastTx, error) {
	// Excess fees correction factor translated to fees using configured gas prices
	// This value is updated by the fee price update routine - making copy for consistency within method
	gasPrice := GetGasPrice()
	walletConfig, err := connectionManager.GetWalletConfig()
	if err != nil {
		return nil, err
	}
	wallet, err := connectionManager.GetWallet()
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
			BaseGas:      walletConfig.BaseGas,
			GasPerByte:   walletConfig.GasPerByte,
			MinGasPrice:  gasPrice,
			OverrideFees: 0,
		},
	}

	txNode, err := connectionManager.GetCurrentTxNode()
	if err != nil {
		log.Error().Err(err).Msg("failed to get current tx node, switching to next")
		txNode, err = connectionManager.SwitchToNextTxNode()
		if err != nil {
			return nil, fmt.Errorf("failed to switch tx node: %w", err)
		}
	}

	for retryCount := int64(0); retryCount <= walletConfig.MaxRetries; retryCount++ {
		log.Debug().Msgf("SendDataWithRetry iteration started (%d/%d)", retryCount, walletConfig.MaxRetries)

		// Create tx without fees to simulate tx creation and get estimated gas and seq number
		txResp, _, errTx := transaction.SendTransactionViaRPC(ctx, txNode.Chain.RPCClient, txNode.ServerAddress, txParams, wallet.GetSequence(), false, req)
		if errTx == nil {
			if txResp != nil {
				log.Info().Msgf("Transaction sent successfully: %v\n", txResp.Hash.String())
			} else {
				log.Error().Msg("Transaction sent successfully but response is nil")
			}
			wallet.IncrementSequence()
			return txResp, nil
		}

		// Handle error on broadcasting
		errorResponse, err := ProcessErrorTx(ctx, errTx, infoMsg, retryCount, walletConfig.MaxRetries, txNode)
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
			// TODO Handle fee and "out of gas" error differently
			got, required, err := parseInsufficientFeeError(errTx.Error(), DEFAULT_BOND_DENOM)
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse insufficient fee error")
			}
			log.Debug().Msgf("Retrying tx with required fee, got %d, required %d", got, required)
			if required > walletConfig.MaxFees.Number.BigInt().Uint64() {
				log.Error().Msgf("Required fee %d is greater than max fees %d", required, walletConfig.MaxFees)
				txParams.GasEstimationConfig.OverrideFees = walletConfig.MaxFees.Number.BigInt().Uint64()
			} else {
				txParams.GasEstimationConfig.OverrideFees = required
			}
			continue
		case ErrorProcessingGas:
			log.Info().Msg("Insufficient gas, marking gas recalculation on tx broadcasting for retrial")
			wanted, used, err := parseGasFromOutOfGasError(errTx.Error())
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse out of gas error")
			}
			newBaseGas := EstimateRequiredBaseGas(wanted, used, txParams.GasEstimationConfig.BaseGas, retryCount)
			log.Debug().Msgf("Retrying tx with required gas, wanted %d, used %d; old base gas %d, new base gas %d", wanted, used, txParams.GasEstimationConfig.BaseGas, newBaseGas)
			txParams.GasEstimationConfig.BaseGas = newBaseGas
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
