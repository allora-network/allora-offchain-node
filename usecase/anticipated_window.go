package usecase

import (
	"allora_offchain_node/lib"
	"time"
	"math"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type AnticipatedWindow struct {
	SoonestTimeForOpenNonceCheck              float64
	SoonestTimeForEndOfWorkerNonceSubmission  float64
	SoonestTimeForEndOfReputerNonceSubmission float64
}

func (window *AnticipatedWindow) BlockIsWithinWindow(block lib.BlockHeight) bool {
	fBlock := float64(block)
	return window.SoonestTimeForOpenNonceCheck <= fBlock && window.SoonestTimeForEndOfWorkerNonceSubmission >= fBlock
}

// `forWorker == true` => Wait until end of worker window, else wait until end of reputer window
func (window *AnticipatedWindow) WaitForNextAnticipatedWindowToStart(currentBlock lib.BlockHeight, epochLength lib.BlockHeight) {
	nextWindowStart := int64(window.SoonestTimeForOpenNonceCheck) + epochLength
	secondsToNextWindowStart := (nextWindowStart - currentBlock) * lib.SECONDS_PER_BLOCK
	time.Sleep(time.Duration(secondsToNextWindowStart) * time.Second)
	return
}

/// Methods Related to AnticipatedWindows but on UseCaseSuite

// Anticipated window is when the current block height is within the soonest start and end times
// at which we begin to check if a nonce is available.
// This window, in blocks, starts at `topic.EpochLastEnded + topic.EpochLength*(1 - config.EarlyArrivalPercent)`
// and ends at `topic.EpochLastEnded + topic.EpochLength*(1 + config.LateArrivalPercent)`
func (suite *UseCaseSuite) WaitWithinAnticipatedWindow() {
	time.Sleep(time.Duration(suite.Node.Wallet.LoopWithinWindowSeconds) * time.Second)
}

// Return the approximate start and end block (as floats) of the next anticipated window.
func (suite *UseCaseSuite) CalcSoonestAnticipatedWindow(topic emissions.Topic, currentBlockHeight lib.BlockHeight) AnticipatedWindow {
	// how many inactive epochs do we have since the last active epoch till now? 
	numInactiveEpochs := (currentBlockHeight - topic.EpochLastEnded) / topic.EpochLength // NOTE: integer devision ignores the remainder
	// how many inactive blocks are there in the inactive epochs? 
	numInactiveBlocks := numInactiveEpochs * topic.EpochLength
	// how many blocks already existed on chain? 
	pastBlocks := topic.EpochLastEnded + numInactiveBlocks

	var (
		soonestWorkerStart int64
		earlyArrival float64
	)
	if pastBlocks + topic.WorkerSubmissionWindow < currentBlockHeight {
		soonestWorkerStart = pastBlocks + topic.EpochLength // look ahead and start in the next anticipated window
		earlyArrival = float64(soonestWorkerStart) - (math.Round((suite.Node.Wallet.EarlyArrivalPercent / 100) * float64(soonestWorkerStart)))
	} else {
		soonestWorkerStart = currentBlockHeight // we are already in the window
		earlyArrival = float64(soonestWorkerStart)
	}
	soonestWorkerEnd := soonestWorkerStart + topic.WorkerSubmissionWindow
	lateArrival := float64(soonestWorkerEnd) + (math.Round((suite.Node.Wallet.LateArrivalPercent / 100) * float64(soonestWorkerEnd)))

	//TODO remove this and create it's own methid for reputer
	soonestReputerEnd := soonestWorkerStart + topic.GroundTruthLag 

	return AnticipatedWindow{
		SoonestTimeForOpenNonceCheck:              earlyArrival,
		SoonestTimeForEndOfWorkerNonceSubmission:  lateArrival,
		SoonestTimeForEndOfReputerNonceSubmission: float64(soonestReputerEnd) * (1.0 + suite.Node.Wallet.LateArrivalPercent),
	}
}
