package lib

import (
	"allora_offchain_node/lib/rpcclient"
	types "allora_offchain_node/lib/types"
	"context"
	"errors"
	"fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
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
	// Transaction params are subject to change among the retries.
	txParams := &types.TransactionParams{
		ChainID:       walletConfig.ChainId,
		Denom:         DEFAULT_BOND_DENOM,
		Prefix:        ADDRESS_PREFIX,
		Sequence:      wallet.GetSequence(),
		AccNum:        wallet.GetAccountNumber(),
		PrivKey:       wallet.GetPrivKey(),
		PubKey:        wallet.GetPubKey(),
		TimeoutHeight: timeoutHeight,
		GasEstimationConfig: types.GasEstimationConfig{
			MinGasPrice:   gasPrice,
			GasAdjustment: walletConfig.GasAdjustment,
			OverrideFees:  0,
		},
		FeeGranterAddress: walletConfig.FeeGranterAddress,
	}

	txNode, err := connectionManager.GetCurrentTxNode()
	if err != nil {
		log.Error().Err(err).Msg("failed to get current tx node, switching to next")
		txNode, err = connectionManager.SwitchToNextTxNode()
		if err != nil {
			return nil, fmt.Errorf("failed to switch tx node: %w", err)
		}
	}
	queryNode, err := connectionManager.GetCurrentQueryNode()
	if err != nil {
		log.Error().Err(err).Msg("failed to get current query node, switching to next")
		queryNode, err = connectionManager.SwitchToNextQueryNode()
		if err != nil {
			return nil, fmt.Errorf("failed to switch query node: %w", err)
		}
	}
	for retryCount := int64(0); retryCount <= walletConfig.MaxRetries; retryCount++ {
		log.Debug().Msgf("SendDataWithRetry iteration started (%d/%d)", retryCount, walletConfig.MaxRetries)
		txParams.Sequence = wallet.GetSequence()
		txResp, _, errTx := SendTransactionViaRPC(ctx, txNode.Chain.RPCClient, txNode.ServerAddress, queryNode, txParams, false, req)
		if errTx == nil {

			if txResp != nil {
				if strings.TrimSpace(txResp.Log) == "" {
					log.Info().Msgf("Transaction sent successfully: %v\n", txResp.Hash.String())
					wallet.IncrementSequence()
					return txResp, nil
				} else {
					log.Warn().Msgf("Transaction sent: %v, but nonempty error log: %s\n", txResp.Hash.String(), txResp.Log)
					// Creating error to process it below.
					errTx = fmt.Errorf("tx failed: %s", txResp.Log)
				}
			}
		}

		if errors.Is(err, ErrTxSimulationError) {
			// simulation failed, let's retry
			continue
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
		case ErrorProcessingResetSequence:
			txParams.Sequence = wallet.GetSequence()
			log.Warn().Msgf("Resetting sequence error on tx to current sequence %d", txParams.Sequence)
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
			log.Info().Msg("Insufficient gas, will re-simulate and retry")
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

// Sends transaction via RPC, and if configured to do so, simulates the tx gas calculation limit via GRPC query.
func SendTransactionViaRPC(ctx context.Context,
	rpcClient *rpcclient.AlloraRPCClient,
	rpcEndpoint string,
	queryNode *NodeConfig,
	txParams *types.TransactionParams,
	waitForTx bool,
	msgs ...sdktypes.Msg,
) (*coretypes.ResultBroadcastTx, string, error) {
	log.Debug().Msgf("Sending transaction via RPC to %s", rpcEndpoint)
	// Build and sign the transaction to get the bytes
	txBytes, err := BuildAndSignTransaction(ctx, queryNode, txParams, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := rpcClient.BroadcastTx(ctx, txBytes, waitForTx)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}

func BuildAndSignTransaction(
	ctx context.Context,
	node *NodeConfig,
	txParams *types.TransactionParams,
	msgs ...sdktypes.Msg,
) ([]byte, error) {
	if err := txParams.Validate(); err != nil {
		return nil, err
	}
	log.Debug().Msgf("Building transaction with sequence %d", txParams.Sequence)
	// Create a new TxBuilder
	txBuilder := txConfig.NewTxBuilder()
	txBuilder.SetTimeoutHeight(txParams.TimeoutHeight)

	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	sigV2 := signing.SignatureV2{
		PubKey:   txParams.PubKey,
		Sequence: txParams.Sequence,
		Data: &signing.SingleSignatureData{ //nolint:exhaustruct
			SignMode: signing.SignMode_SIGN_MODE_DIRECT,
		},
	}
	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	// Gas simulation
	unsignedTx, err := txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}
	gas, err := node.SimulateTxWithRetry(ctx, unsignedTx)
	if err != nil {
		return nil, err
	}
	if txParams.GasEstimationConfig.GasAdjustment > 0 {
		gas = uint64(float64(gas) * txParams.GasEstimationConfig.GasAdjustment)
	}
	txBuilder.SetGasLimit(gas)

	// Calculate fees for tx, potentially override with a fixed value
	var fees math.Int
	if txParams.GasEstimationConfig.OverrideFees > 0 {
		// Set the gas price to the override value
		fees = math.NewIntFromUint64(txParams.GasEstimationConfig.OverrideFees)
	} else {
		// Calculate using gas limit and min gas price
		fees, err = rpcclient.CalculateFees(gas, txParams.GasEstimationConfig.MinGasPrice)
		if err != nil {
			return nil, err
		}
	}
	// Set fees for tx
	txBuilder.SetFeeAmount(sdktypes.NewCoins(sdktypes.NewCoin(txParams.Denom, fees)))

	// Set fee granter (optional)
	if txParams.FeeGranterAddress != "" {
		granterAddr, err := sdktypes.AccAddressFromBech32(txParams.FeeGranterAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse fee granter address %v: %w", txParams.FeeGranterAddress, err)
		}
		txBuilder.SetFeeGranter(granterAddr)
	}

	signerData := authsigning.SignerData{ //nolint:exhaustruct
		ChainID:       txParams.ChainID,
		AccountNumber: txParams.AccNum,
		Sequence:      txParams.Sequence,
	}

	// Sign the transaction with the private key
	sigV2, err = tx.SignWithPrivKey(
		ctx,
		signing.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder,
		txParams.PrivKey,
		txConfig,
		txParams.Sequence,
	)
	if err != nil {
		return nil, err
	}

	// Set the signed signature back to the txBuilder
	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	// Encode the transaction
	return txConfig.TxEncoder()(txBuilder.GetTx())
}
