package usecase

import (
	"allora_offchain_node/lib"
	"time"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type AnticipatedWindow struct {
	SoonestTimeForOpenNonceCheck              float64
	SoonestTimeForEndOfWorkerNonceSubmission  float64
	SoonestTimeForEndOfReputerNonceSubmission float64
}

/// Interface Methods

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
// First, find the smallest integer N such that `EpochLastEnded + N * EpochLength >= now()` analytically.
// Then this begins the next anticipated window.
// We do this because the topic could be inactive for some epochs.
// We then apply the early arrival and late arrival percentages to this window.
// If the current block height is within this window, we check for a nonce => return true.
// Essentially the AnticipatedWindow factory.
func (suite *UseCaseSuite) CalcSoonestAnticipatedWindow(topic emissions.Topic, currentBlockHeight lib.BlockHeight) AnticipatedWindow {
	numInactiveEpochs := (currentBlockHeight - topic.EpochLastEnded) / topic.EpochLength // how many inactive epochs do we have since the last active epoch till now?

	// TODO Remove this when WindowLength added to the topic query response
	const WindowLength int64 = 2

	soonestStart := topic.EpochLastEnded + numInactiveEpochs*topic.EpochLength - currentBlockHeight // start of the next anticipated window, considering how many inactive epochs we already have. if negative, we are already in the window and the result is how many blocks away from the start of next epoch
	soonestWorkerEnd := soonestStart + WindowLength
	soonestReputerEnd := soonestStart + topic.EpochLength

	return AnticipatedWindow{
		SoonestTimeForOpenNonceCheck:              float64(soonestStart) * (1.0 - suite.Node.Wallet.EarlyArrivalPercent),
		SoonestTimeForEndOfWorkerNonceSubmission:  float64(soonestWorkerEnd) * (1.0 + suite.Node.Wallet.LateArrivalPercent),
		SoonestTimeForEndOfReputerNonceSubmission: float64(soonestReputerEnd) * (1.0 + suite.Node.Wallet.LateArrivalPercent),
	}
}
