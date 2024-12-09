package lib

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	errorsmod "cosmossdk.io/errors"
	emissions "github.com/allora-network/allora-chain/x/emissions/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/rs/zerolog/log"
	feemarkettypes "github.com/skip-mev/feemarket/x/feemarket/types"
)

// Error codes for the module
const ErrorCodespace = "allora-offchain-lib"

var (
	ErrTooManyRequests  = errorsmod.Register(ErrorCodespace, 1, "too many requests")
	ErrNotEnoughBalance = errorsmod.Register(ErrorCodespace, 2, "not enough balance")
	ErrNotRegistered    = errorsmod.Register(ErrorCodespace, 3, "not registered")
	ErrStakeBelowMin    = errorsmod.Register(ErrorCodespace, 4, "stake below minimum")
)

const ErrorMessageAbciErrorCodeMarker = "error code:"
const ErrorMessageDataAlreadySubmitted = "already submitted"
const ErrorMessageCannotUpdateEma = "cannot update EMA"
const ErrorMessageWaitingForNextBlock = "waiting for next block" // This means tx is accepted in mempool but not yet included in a block
const ErrorMessageAccountSequenceMismatch = "account sequence mismatch"
const ErrorMessageTimeoutHeight = "timeout height"
const ErrorMessageNotPermittedToSubmitPayload = "not permitted to submit payload"
const ErrorMessageNotPermittedToAddStake = "not permitted to add stake"

const ExcessCorrectionInGas = 20000

// Error processing types
// - "continue", nil: tx was not successful, but special error type. Handled, ready for retry
// - "ok", nil: tx was successful, error handled and not re-raised
// - "error", error: tx failed, with regular error type
// - "fees": tx failed, because of insufficient fees
// - "failure": tx failed, and should not be retried anymore
// - "switch": tx failed, and should be retried with a different node
const ErrorProcessingContinue = "continue"
const ErrorProcessingOk = "ok"
const ErrorProcessingFees = "fees"
const ErrorProcessingError = "error"
const ErrorProcessingFailure = "failure"
const ErrorProcessingSwitchingNode = "switch"

// HTTP status codes that trigger node switching
var HTTPStatusCodeCodesSwitchingNode = map[int]bool{
	429: true, // Too Many Requests
}

// calculateExponentialBackoffDelay returns a duration based on retry count and base delay
func calculateExponentialBackoffDelaySeconds(baseDelay int64, retryCount int64) int64 {
	return int64(math.Pow(float64(baseDelay), float64(retryCount)))
}

// processError handles the error messages.
func ProcessErrorTx(ctx context.Context, err error, infoMsg string, retryCount int64, node *NodeConfig) (string, error) {
	if strings.Contains(err.Error(), ErrorMessageAbciErrorCodeMarker) {
		re := regexp.MustCompile(`error code: '(\d+)'`)
		matches := re.FindStringSubmatch(err.Error())
		if len(matches) == 2 {
			errorCode, parseErr := strconv.Atoi(matches[1])
			if parseErr != nil {
				log.Error().Err(parseErr).Str("msg", infoMsg).Msg("Failed to parse ABCI error code")
			} else {
				switch errorCode {
				case int(sdkerrors.ErrMempoolIsFull.ABCICode()):
					log.Warn().
						Err(err).
						Str("msg", infoMsg).
						Msg("Mempool is full, retrying with exponential backoff")
					delay := calculateExponentialBackoffDelaySeconds(node.Wallet.RetryDelay, retryCount)
					if DoneOrWait(ctx, delay) {
						return ErrorProcessingError, ctx.Err()
					}
					return ErrorProcessingContinue, nil
				case int(sdkerrors.ErrWrongSequence.ABCICode()), int(sdkerrors.ErrInvalidSequence.ABCICode()):
					log.Warn().
						Err(err).
						Str("msg", infoMsg).
						Int64("delay", node.Wallet.AccountSequenceRetryDelay).
						Msg("Account sequence mismatch detected, retrying with fixed delay")
					// Wait a fixed block-related waiting time
					if DoneOrWait(ctx, node.Wallet.AccountSequenceRetryDelay) {
						return ErrorProcessingError, ctx.Err()
					}
					return ErrorProcessingContinue, nil
				case int(sdkerrors.ErrInsufficientFee.ABCICode()):
					log.Info().
						Err(err).
						Str("msg", infoMsg).
						Msg("Insufficient fees")
					return ErrorProcessingFees, nil
				case int(feemarkettypes.ErrNoFeeCoins.ABCICode()):
					log.Info().
						Err(err).
						Str("msg", infoMsg).
						Msg("No fee coins")
					return ErrorProcessingFees, nil
				case int(sdkerrors.ErrTxTooLarge.ABCICode()):
					return ErrorProcessingError, errorsmod.Wrapf(err, "tx too large")
				case int(sdkerrors.ErrTxInMempoolCache.ABCICode()):
					return ErrorProcessingError, errorsmod.Wrapf(err, "tx already in mempool cache")
				case int(sdkerrors.ErrInvalidChainID.ABCICode()):
					return ErrorProcessingError, errorsmod.Wrapf(err, "invalid chain-id")
				case int(sdkerrors.ErrTxTimeoutHeight.ABCICode()):
					return ErrorProcessingFailure, errorsmod.Wrapf(err, "tx timeout height")
				case int(emissions.ErrWorkerNonceWindowNotAvailable.ABCICode()):
					log.Warn().
						Err(err).
						Str("msg", infoMsg).
						Msg("Worker window not available, retrying with exponential backoff")
					delay := calculateExponentialBackoffDelaySeconds(node.Wallet.RetryDelay, retryCount)
					if DoneOrWait(ctx, delay) {
						return ErrorProcessingError, ctx.Err()
					}
					return ErrorProcessingContinue, nil
				case int(emissions.ErrReputerNonceWindowNotAvailable.ABCICode()):
					log.Warn().
						Err(err).
						Str("msg", infoMsg).
						Msg("Reputer window not available, retrying with exponential backoff")
					delay := calculateExponentialBackoffDelaySeconds(node.Wallet.RetryDelay, retryCount)
					if DoneOrWait(ctx, delay) {
						return ErrorProcessingError, ctx.Err()
					}
					return ErrorProcessingContinue, nil
				default:
					log.Info().Int("errorCode", errorCode).Str("msg", infoMsg).Msg("ABCI error, but not special case - regular retry")
				}
			}
		} else {
			log.Warn().Str("msg", infoMsg).Msg("Unmatched error format, cannot classify as ABCI error")
		}
	}

	// Check if error is HTTP status code
	if statusCode, statusMessage, error := ParseHTTPStatus(err.Error()); error == nil {
		log.Warn().Int("statusCode", statusCode).Str("statusMessage", statusMessage).Str("msg", infoMsg).Msg("HTTP status code detected")
		if statusCode, statusMessage, err := ParseHTTPStatus(err.Error()); err == nil {
			log.Warn().Int("statusCode", statusCode).Str("statusMessage", statusMessage).Str("msg", infoMsg).Msg("HTTP status code detected")

			if HTTPStatusCodeCodesSwitchingNode[statusCode] {
				log.Warn().
					Int("statusCode", statusCode).
					Str("msg", infoMsg).
					Msg("HTTP status error code detected, switching to next node")
				return ErrorProcessingSwitchingNode, ErrTooManyRequests
			}
		}
	}

	// NOT ABCI error code: keep on checking for specially handled error types
	if strings.Contains(err.Error(), ErrorMessageAccountSequenceMismatch) {
		log.Warn().
			Err(err).
			Str("msg", infoMsg).
			Int64("delay", node.Wallet.AccountSequenceRetryDelay).
			Msg("Account sequence mismatch detected, re-fetching sequence")
		if DoneOrWait(ctx, node.Wallet.AccountSequenceRetryDelay) {
			return ErrorProcessingError, ctx.Err()
		}
		return ErrorProcessingContinue, nil
	} else if strings.Contains(err.Error(), ErrorMessageWaitingForNextBlock) {
		log.Warn().Err(err).Str("msg", infoMsg).Msg("Tx accepted in mempool, it will be included in the following block(s) - not retrying")
		return ErrorProcessingOk, nil
	} else if strings.Contains(err.Error(), ErrorMessageDataAlreadySubmitted) || strings.Contains(err.Error(), ErrorMessageCannotUpdateEma) {
		log.Warn().Err(err).Str("msg", infoMsg).Msg("Already submitted data for this epoch.")
		return ErrorProcessingOk, nil
	} else if strings.Contains(err.Error(), ErrorMessageTimeoutHeight) {
		log.Warn().Err(err).Str("msg", infoMsg).Msg("Tx failed because of timeout height")
		return ErrorProcessingFailure, err
	} else if strings.Contains(err.Error(), ErrorMessageNotPermittedToSubmitPayload) {
		log.Warn().Err(err).Str("msg", infoMsg).Msg("Actor is not permitted to submit payload")
		return ErrorProcessingFailure, err
	} else if strings.Contains(err.Error(), ErrorMessageNotPermittedToAddStake) {
		log.Warn().Err(err).Str("msg", infoMsg).Msg("Actor is not permitted to add stake")
		return ErrorProcessingFailure, err
	}

	return ErrorProcessingError, errorsmod.Wrapf(err, "failed to process error")
}

// func ProcessErrorQuery(ctx context.Context, err error, infoMsg string, retryCount int64, node *NodeConfig) (string, error) {

// }

// ParseStatus parses a status code and message from a given text string.
func ParseHTTPStatus(input string) (int, string, error) {
	// Regular expression to match "Status: <code> <message>" or similar patterns in text
	re := regexp.MustCompile(`(?i)status:\s*(\d+)\s*([^,]*)`)

	matches := re.FindStringSubmatch(input)
	if len(matches) < 3 {
		return 0, "", errors.New("invalid input format")
	}

	// Parse the status code
	statusCode, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", fmt.Errorf("invalid status code: %v", err)
	}

	// Get the status message
	statusMessage := strings.TrimSpace(matches[2])

	return statusCode, statusMessage, nil
}

// Returns true if the error is a switching-node error
func IsErrorSwitchingNode(err error) bool {
	return errors.Is(err, ErrTooManyRequests)
}
