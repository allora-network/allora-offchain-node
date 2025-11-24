package rpcclient

import (
	"context"
	"sync"

	cmtjson "github.com/cometbft/cometbft/libs/json"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/rs/zerolog"
)

// WSEvents allows subscribing to comet web socket events, managing both the underlying socket and event routing.
type WSEvents struct {
	logger zerolog.Logger
	ws     *jsonrpcclient.WSClient

	mtx           sync.RWMutex
	subscriptions map[string]chan ctypes.ResultEvent
}

func NewWSEvents(remote, wsEndpoint string, logger zerolog.Logger) (*WSEvents, error) {
	w := &WSEvents{
		logger:        logger,
		ws:            nil,
		mtx:           sync.RWMutex{},
		subscriptions: make(map[string]chan ctypes.ResultEvent),
	}

	var err error
	w.ws, err = jsonrpcclient.NewWS(remote, wsEndpoint, jsonrpcclient.OnReconnect(func() {
		w.redoSubscriptions()
	}), jsonrpcclient.MaxReconnectAttempts(8))
	if err != nil {
		return nil, err
	}

	if err := w.ws.Start(); err != nil {
		return nil, err
	}

	return w, nil
}

// Listen start the web socket and route received events to their associated channels. It is blocking until the web socket
// is stopped.
func (w *WSEvents) Listen() {
	for {
		select {
		case resp, ok := <-w.ws.ResponsesCh:
			if !ok {
				return
			}

			if resp.Error != nil {
				w.logger.Err(resp.Error).Msg("WS response error")
				continue
			}

			result := new(ctypes.ResultEvent)
			err := cmtjson.Unmarshal(resp.Result, result)
			if err != nil {
				w.logger.Err(err).Msg("Failed to unmarshal WS response")
				continue
			}

			w.mtx.RLock()
			if out, ok := w.subscriptions[result.Query]; ok {
				out <- *result
			}
			w.mtx.RUnlock()
		}
	}
}

// Stop the web socket client, in case of error forces the socket response channel closing to ensure the listening
// will stop as well.
func (w *WSEvents) Stop() {
	if err := w.ws.Stop(); err != nil {
		w.logger.Err(err).Msg("Couldn't stop WSClient")
		return
	}
}

func (w *WSEvents) Subscribe(ctx context.Context, query string) (out <-chan ctypes.ResultEvent, err error) {
	if err := w.ws.Subscribe(ctx, query); err != nil {
		return nil, err
	}

	outc := make(chan ctypes.ResultEvent, 1)
	w.mtx.Lock()
	w.subscriptions[query] = outc
	w.mtx.Unlock()

	return outc, nil
}

func (w *WSEvents) Unsubscribe(ctx context.Context, query string) error {
	if err := w.ws.Unsubscribe(ctx, query); err != nil {
		return err
	}

	w.mtx.Lock()
	_, ok := w.subscriptions[query]
	if ok {
		delete(w.subscriptions, query)
	}
	w.mtx.Unlock()

	return nil
}

func (w *WSEvents) redoSubscriptions() {
	w.mtx.RLock()
	defer w.mtx.RUnlock()

	for q := range w.subscriptions {
		if err := w.ws.Subscribe(context.Background(), q); err != nil {
			w.logger.Err(err).Msg("Failed to resubscribe after ws reconnect")
			w.Stop()
		}
	}
}
