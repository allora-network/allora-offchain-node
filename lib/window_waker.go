package lib

import (
	"allora_offchain_node/lib/rpcclient"
	"context"
	"fmt"
	"sync"
	"time"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	comettypes "github.com/cometbft/cometbft/types"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

type WindowWakeFn func(context.Context, emissionstypes.Nonce, int64)

type WindowType int

const (
	ReputerWindow WindowType = iota
	WorkerWindow
)

// WindowWaker allows to watch submissions windows opening events through the comet rpc web socket, and execute the
// attached waking logic. It allows to watch either reputer or worker windows for a specific topic id.
type WindowWaker struct {
	windowType        WindowType
	connectionManager ConnectionManagerInterface
	logger            zerolog.Logger
	topicID           emissionstypes.TopicId
	wakeFn            WindowWakeFn

	ws       *rpcclient.WSEvents
	wsQuery  string
	wsWG     *sync.WaitGroup
	wsStopCh chan struct{}
}

func NewReputerWindowWaker(
	connectionManager ConnectionManagerInterface,
	ws *rpcclient.WSEvents,
	topicID emissionstypes.TopicId,
	wakeFn WindowWakeFn,
) *WindowWaker {
	return &WindowWaker{
		windowType:        ReputerWindow,
		connectionManager: connectionManager,
		logger:            log.With().Uint64("topicId", topicID).Int("windowType", int(ReputerWindow)).Logger(),
		topicID:           topicID,
		wakeFn:            wakeFn,
		ws:                ws,
		wsQuery:           "",
		wsWG:              nil,
		wsStopCh:          nil,
	}
}

func NewWorkerWindowWaker(
	connectionManager ConnectionManagerInterface,
	ws *rpcclient.WSEvents,
	topicID emissionstypes.TopicId,
	wakeFn WindowWakeFn,
) *WindowWaker {
	return &WindowWaker{
		windowType:        WorkerWindow,
		connectionManager: connectionManager,
		logger:            log.With().Uint64("topicId", topicID).Int("windowType", int(WorkerWindow)).Logger(),
		topicID:           topicID,
		wakeFn:            wakeFn,
		ws:                ws,
		wsQuery:           "",
		wsWG:              nil,
		wsStopCh:          nil,
	}
}

// Start launches the waker by subscribing to the related ws event, and checking if a window is already open (in which
// case the event has already been missed)
func (ww *WindowWaker) Start(ctx context.Context) error {
	ww.logger.Info().Msg("Starting window waker")

	latestBlock, err := RunWithNodeRetry(
		ctx,
		ww.connectionManager,
		func(node *NodeConfig) (*coretypes.ResultBlock, error) {
			return node.Chain.RPCClient.Client.Block(ctx, nil)
		},
		"get latest block",
		RPC_MODE,
	)
	if err != nil {
		return err
	}
	latestHeight := latestBlock.Block.Height

	if err := ww.subscribeToWindowEvents(ctx); err != nil {
		return err
	}

	nonce, err := ww.getOpenNonceAtHeight(ctx, latestHeight)
	if err != nil {
		return err
	}
	if nonce != nil {
		ww.logger.Info().Int64("nonce", *nonce).Msg("Currently opened window")
		ww.wakeFn(ctx, emissionstypes.Nonce{BlockHeight: *nonce}, latestHeight)
	}

	return nil
}

func (ww *WindowWaker) subscribeToWindowEvents(ctx context.Context) error {
	evtsChan, err := ww.ws.Subscribe(ctx, ww.prepareWSQuery())
	if err != nil {
		return err
	}
	ww.wsStopCh = make(chan struct{})
	ww.wsWG = &sync.WaitGroup{}

	ww.wsWG.Add(1)
	go func() {
		defer ww.wsWG.Done()

		for {
			select {
			case <-ww.wsStopCh:
				ww.logger.Info().Msg("Stopping window waker")
				return
			case event := <-evtsChan:
				blockEvts, ok := event.Data.(comettypes.EventDataNewBlockEvents)
				if !ok {
					ww.logger.Info().Msg("Received non comettypes.EventDataNewBlockEvents from comet ws")
					continue
				}

				var nonce *int64
				for _, abciEvt := range blockEvts.Events {
					// HACK: Remove comet 'mode' additional attribute
					for i, attribute := range abciEvt.Attributes {
						if attribute.Key == "mode" {
							abciEvt.Attributes = append(abciEvt.Attributes[:i], abciEvt.Attributes[i+1:]...)
							break
						}
					}

					evt, err := cosmostypes.ParseTypedEvent(abciEvt)
					if err != nil {
						continue
					}

					if ww.windowType == ReputerWindow {
						windowEvt, ok := evt.(*emissionstypes.EventReputerSubmissionWindowOpened)
						if ok && windowEvt.TopicId == ww.topicID {
							nonce = &windowEvt.NonceBlockHeight
							break
						}
					} else {
						windowEvt, ok := evt.(*emissionstypes.EventWorkerSubmissionWindowOpened)
						if ok && windowEvt.TopicId == ww.topicID {
							nonce = &windowEvt.NonceBlockHeight
							break
						}
					}
				}
				if nonce != nil {
					ww.logger.Info().Int64("nonce", *nonce).Msg("New window open event")
					ww.wakeFn(ctx, emissionstypes.Nonce{BlockHeight: *nonce}, blockEvts.Height)
				} else {
					ww.logger.Error().Msg("Couldn't lookup window event in block events")
				}
			}
		}
	}()

	return nil
}

func (ww *WindowWaker) getOpenNonceAtHeight(ctx context.Context, height int64) (*int64, error) {
	heightCtx := metadata.AppendToOutgoingContext(ctx, "x-cosmos-block-height", fmt.Sprintf("%d", height))
	var nonce *int64
	if ww.windowType == WorkerWindow {
		window, err := RunWithNodeRetry(
			ctx,
			ww.connectionManager,
			func(node *NodeConfig) (*emissionstypes.GetWorkerSubmissionWindowStatusResponse, error) {
				return emissionstypes.NewQueryServiceClient(node.Chain.GRPCClient).
					GetWorkerSubmissionWindowStatus(
						heightCtx,
						&emissionstypes.GetWorkerSubmissionWindowStatusRequest{TopicId: ww.topicID}, //nolint: exhaustruct
					)
			},
			"get current window status",
			GRPC_MODE,
		)
		if err != nil {
			return nil, err
		}
		if window.IsOpen {
			nonce = &window.CurrentNonceBlockHeight
		}
	} else {
		window, err := RunWithNodeRetry(
			ctx,
			ww.connectionManager,
			func(node *NodeConfig) (*emissionstypes.GetReputerSubmissionWindowStatusResponse, error) {
				return emissionstypes.NewQueryServiceClient(node.Chain.GRPCClient).
					GetReputerSubmissionWindowStatus(
						heightCtx,
						&emissionstypes.GetReputerSubmissionWindowStatusRequest{TopicId: ww.topicID}, //nolint: exhaustruct
					)
			},
			"get current window status",
			GRPC_MODE,
		)
		if err != nil {
			return nil, err
		}
		if window.IsOpen {
			nonce = &window.CurrentNonceBlockHeight
		}
	}
	return nonce, nil
}

func (ww *WindowWaker) prepareWSQuery() string {
	var qFormat string
	if ww.windowType == ReputerWindow {
		qFormat = "tm.event='NewBlockEvents' AND emissions.v9.EventReputerSubmissionWindowOpened.topic_id='\"%d\"'"
	} else {
		qFormat = "tm.event='NewBlockEvents' AND emissions.v9.EventWorkerSubmissionWindowOpened.topic_id='\"%d\"'"
	}
	ww.wsQuery = fmt.Sprintf(qFormat, ww.topicID)

	return ww.wsQuery
}

func (ww *WindowWaker) Stop(ctx context.Context) {
	ww.wsStopCh <- struct{}{}
	if err := ww.ws.Unsubscribe(ctx, ww.wsQuery); err != nil {
		ww.logger.Err(err).Msg("Failed to unsubscribe comet ws stopping window waker")
	}

	ww.wsWG.Wait()
}

type SubmissionWindowManager struct {
	connectionManager ConnectionManagerInterface
	ws                *rpcclient.WSEvents
	wsRunning         bool
	logger            zerolog.Logger
	stopCh            chan struct{}

	mtx    sync.Mutex
	wakers []*WindowWaker
}

// NewSubmissionWindowManager creates a new SubmissionWindowManager, initiating the underlying used web socket for
// subscribing to events, launching the listening routine.
func NewSubmissionWindowManager(connectionManager ConnectionManagerInterface) (*SubmissionWindowManager, error) {
	node, err := connectionManager.GetCurrentTxNode()
	if err != nil {
		return nil, err
	}

	ws, err := rpcclient.NewWSEvents(node.ServerAddress, "/websocket", log.With().Logger())
	if err != nil {
		return nil, err
	}

	swm := &SubmissionWindowManager{
		connectionManager: connectionManager,
		ws:                ws,
		wsRunning:         true,
		logger:            log.With().Logger(),
		stopCh:            make(chan struct{}),
		mtx:               sync.Mutex{},
		wakers:            nil,
	}

	go swm.wsListen()

	return swm, nil
}

func (swm *SubmissionWindowManager) WakeOnWorkerWindowOpen(ctx context.Context, topicID emissionstypes.TopicId, wakeFn WindowWakeFn) error {
	swm.mtx.Lock()
	defer swm.mtx.Unlock()

	if !swm.wsRunning {
		return fmt.Errorf("ws not running")
	}

	ww := NewWorkerWindowWaker(swm.connectionManager, swm.ws, topicID, wakeFn)
	if err := ww.Start(ctx); err != nil {
		return err
	}

	swm.wakers = append(swm.wakers, ww)
	return nil
}

func (swm *SubmissionWindowManager) WakeOnReputerWindowOpen(ctx context.Context, topicID emissionstypes.TopicId, wakeFn WindowWakeFn) error {
	swm.mtx.Lock()
	defer swm.mtx.Unlock()

	if !swm.wsRunning {
		return fmt.Errorf("ws not running")
	}

	ww := NewReputerWindowWaker(swm.connectionManager, swm.ws, topicID, wakeFn)
	if err := ww.Start(ctx); err != nil {
		return err
	}

	swm.wakers = append(swm.wakers, ww)
	return nil
}

// wsListen let the WSEvent listener to the websocket events in a blocking way, stopping wakers on exit.
func (swm *SubmissionWindowManager) wsListen() {
	swm.ws.Listen()

	swm.logger.Info().Msg("WS closed, stopping wakers")
	swm.mtx.Lock()
	swm.stopWakers()
	swm.wsRunning = false
	swm.mtx.Unlock()

	swm.stopCh <- struct{}{}
}

func (swm *SubmissionWindowManager) stopWakers() {
	ctx, cl := context.WithTimeout(context.Background(), 5*time.Second)
	defer cl()

	var wg sync.WaitGroup
	for _, w := range swm.wakers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Stop(ctx)
		}()
	}

	wg.Wait()
}

// Join waits for the underlying routine termination, denoting that the web socket is closed as of every WindowWaker.
// Termination can be provoked by either a call to Stop, or if the web socket connection was lost without being able to
// reconnect.
func (swm *SubmissionWindowManager) Join() {
	<-swm.stopCh
}

// Stop the web socket and all WindowWaker in a non-blocking manner (i.e. use Join to sync with underlying routines).
// It stops the web socket client which will stop wakers in cascade.
func (swm *SubmissionWindowManager) Stop() {
	swm.ws.Stop()
}
