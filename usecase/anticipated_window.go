package usecase

import (
	"allora_offchain_node/lib"
	"math"
	"time"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
	"github.com/rs/zerolog/log"
)

const DELAY_CORRECTION_FACTOR = 0.6
const MIN_LIMIT_BLOCKS = 3

type AnticipatedWindow struct {
	SoonestTimeForOpenNonceCheck                float64
	SoonestTimeForEndOfWorkerNonceSubmission    float64
	SoonestTimeForStartOfReputerNonceSubmission float64 `json:"SoonestTimeForStartOfReputerNonceSubmission,omitempty"`
	SoonestTimeForEndOfReputerNonceSubmission   float64 `json:"SoonestTimeForEndOfReputerNonceSubmission,omitempty"`
}

func (window *AnticipatedWindow) BlockIsWithinWindow(block lib.BlockHeight) bool {
	fBlock := float64(block)
	return window.SoonestTimeForOpenNonceCheck <= fBlock && window.SoonestTimeForEndOfWorkerNonceSubmission >= fBlock
}

func (window *AnticipatedWindow) BlockIsWithinReputerWindow(block lib.BlockHeight) bool {
	fBlock := float64(block)
	return window.SoonestTimeForStartOfReputerNonceSubmission <= fBlock && window.SoonestTimeForEndOfReputerNonceSubmission >= fBlock
}

// Waits until the next window starts, given the current block height and the next window start block height,
// with a correction factor applied to the waiting time.
func (window *AnticipatedWindow) WaitForNextWindowToStart(currentBlock lib.BlockHeight, nextWindowStart lib.BlockHeight) {
	waitingTimeInBlocks := nextWindowStart - currentBlock
	if waitingTimeInBlocks == 0 {
		log.Debug().Msg("No difference in time, waiting in 1s")
		time.Sleep(time.Duration(1) * time.Second)
	} else if waitingTimeInBlocks < MIN_LIMIT_BLOCKS {
		waitingTimeSeconds := waitingTimeInBlocks * lib.SECONDS_PER_BLOCK
		log.Debug().
			Int64("waitingTimeInSeconds", waitingTimeSeconds).
			Msg("Waiting time is less than the minimum limit, sleeping...")
		time.Sleep(time.Duration(waitingTimeSeconds) * time.Second)
	} else {
		correctedNumberOfWaitingBlocks := int64(float64(waitingTimeInBlocks) * DELAY_CORRECTION_FACTOR)
		secondsToNextWindowStart := correctedNumberOfWaitingBlocks * lib.SECONDS_PER_BLOCK
		log.Debug().
			Int64("currentBlock", int64(currentBlock)).
			Int64("nextWindowStart", int64(nextWindowStart)).
			Int64("waitingTimeInBlocks", int64(waitingTimeInBlocks)).
			Int64("secondsToNextWindowStart", int64(secondsToNextWindowStart)).
			Msg("Waiting for next window to start")
		time.Sleep(time.Duration(secondsToNextWindowStart) * time.Second)
	}
}

func (window *AnticipatedWindow) WaitForNextAnticipatedWindowToStart(currentBlock lib.BlockHeight, epochLength lib.BlockHeight) {
	// TODO Apply a correction factor to the next window start time
	nextWindowStart := int64(window.SoonestTimeForOpenNonceCheck) + epochLength
	secondsToNextWindowStart := (nextWindowStart - currentBlock) * lib.SECONDS_PER_BLOCK
	time.Sleep(time.Duration(secondsToNextWindowStart) * time.Second)
	return
}

func (window *AnticipatedWindow) WaitForNextReputerAnticipatedWindowToStart(topic emissions.Topic, nonce lib.BlockHeight, currentBlock lib.BlockHeight) {
	// TODO Apply a correction factor to the next window start time
	nextWindowStart := nonce + topic.GroundTruthLag
	secondsToNextWindowStart := (nextWindowStart - currentBlock) * lib.SECONDS_PER_BLOCK
	time.Sleep(time.Duration(secondsToNextWindowStart) * time.Second)
	return
}

// Anticipated window is when the current block height is within the soonest start and end times
// at which we begin to check if a nonce is available.
func (suite *UseCaseSuite) WaitWithinAnticipatedWindow() {
	time.Sleep(time.Duration(suite.Node.Wallet.LoopWithinWindowSeconds) * time.Second)
}

// Return the approximate start and end block (as floats) of the next anticipated window.
func (window AnticipatedWindow) CalcWorkerSoonestAnticipatedWindow(suite *UseCaseSuite, topic emissions.Topic, currentBlockHeight lib.BlockHeight) AnticipatedWindow {
	// how many inactive epochs do we have since the last active epoch till now?
	numInactiveEpochs := (currentBlockHeight - topic.EpochLastEnded) / topic.EpochLength // NOTE: integer devision ignores the remainder
	// how many inactive blocks are there in the inactive epochs?
	numInactiveBlocks := numInactiveEpochs * topic.EpochLength
	// how many blocks already existed on chain?
	pastBlocks := topic.EpochLastEnded + numInactiveBlocks

	var (
		soonestWorkerStart int64
		earlyArrival       float64
	)
	if pastBlocks+topic.WorkerSubmissionWindow < currentBlockHeight {
		soonestWorkerStart = pastBlocks + topic.EpochLength // look ahead and start in the next anticipated window

		earlyArrival = float64(soonestWorkerStart) - (math.Round((suite.Node.Wallet.EarlyArrivalPercent / 100) * float64(soonestWorkerStart)))
	} else {
		soonestWorkerStart = currentBlockHeight // we are already in the window
		earlyArrival = float64(soonestWorkerStart)
	}
	soonestWorkerEnd := soonestWorkerStart + topic.WorkerSubmissionWindow
	lateArrival := float64(soonestWorkerEnd) + (math.Round((suite.Node.Wallet.LateArrivalPercent / 100) * float64(soonestWorkerEnd)))

	return AnticipatedWindow{
		SoonestTimeForOpenNonceCheck:             earlyArrival,
		SoonestTimeForEndOfWorkerNonceSubmission: lateArrival,
	}
}

func (window *AnticipatedWindow) CalcReputerSoonestAnticipatedWindow(topic emissions.Topic, openNonce lib.BlockHeight) *AnticipatedWindow {
	// asumming there is no need for early or late arrival since the window (epoch length) is big enough to submit reputation
	soonestReputerStart := openNonce + topic.GroundTruthLag
	soonestReputerEnd := soonestReputerStart + topic.EpochLength

	window.SoonestTimeForStartOfReputerNonceSubmission = float64(soonestReputerStart)
	window.SoonestTimeForEndOfReputerNonceSubmission = float64(soonestReputerEnd)

	return window
}
