package grpcclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"

	"allora_offchain_node/metrics"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type backoffConfig struct {
	initial    time.Duration
	max        time.Duration
	maxRetries int
	jitterFrac float64 // fraction of the backoff to use for jitter
}

func newBackoffConfig() backoffConfig {
	return backoffConfig{
		initial:    1 * time.Second,
		max:        30 * time.Second,
		maxRetries: 5,
		jitterFrac: 0.2, // 20% jitter
	}
}

func (bc *backoffConfig) nextBackoff(current time.Duration) time.Duration {
	// Calculate base backoff with exponential increase
	next := time.Duration(math.Min(float64(current*2), float64(bc.max)))

	// Apply jitter: randomly subtract up to jitterFrac of the duration
	jitterRange := float64(next) * bc.jitterFrac

	// Generate cryptographically secure random number between 0 and jitterRange
	maxJitter := big.NewInt(int64(jitterRange))
	randomBig, err := rand.Int(rand.Reader, maxJitter)
	if err != nil {
		// If we fail to generate random jitter, just return the next backoff without jitter
		log.Warn().Err(err).Msg("Failed to generate jitter, continuing without it")
		return next
	}

	jitter := time.Duration(randomBig.Int64())
	return next - jitter
}

func monitorGRPCConnection(ctx context.Context, grpcConnection *grpc.ClientConn, grpcEndpoint string) {
	if grpcConnection == nil {
		log.Error().Msg("nil gRPC connection provided to monitor")
		return
	}

	ticker := time.NewTicker(5 * time.Second) // halved from 10s — faster reaction to upstream stream resets
	defer ticker.Stop()

	bc := newBackoffConfig()
	retryCount := 0
	backoff := bc.initial
	// Track how long we've observed Idle. grpc-go transitions to Idle when
	// the underlying HTTP/2 stream is killed but the conn object hasn't been
	// declared TransientFailure yet (notably after CDN/LB half-closes). A
	// transient Idle is normal (just no traffic); a sustained one means we're
	// stuck and the next RPC will fail.
	const idleGraceTicks = 6 // 6 ticks * 5s = 30s of idle before we treat it as stuck
	idleTicks := 0

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Shutting down gRPC monitoring goroutine")
			return
		case <-ticker.C:
			state := grpcConnection.GetState()

			// Track Idle separately. If we stay Idle for too long, treat it
			// the same as a TransientFailure for reconnect purposes.
			if state == connectivity.Idle {
				idleTicks++
				if idleTicks < idleGraceTicks {
					continue
				}
				log.Warn().Int("idle_seconds", idleTicks*5).Str("endpoint", grpcEndpoint).Msg("gRPC connection has been Idle past grace window, forcing reconnect")
			} else {
				idleTicks = 0
			}

			// Only attempt reconnection for failed states (plus sustained Idle handled above)
			if state != connectivity.TransientFailure && state != connectivity.Shutdown && state != connectivity.Idle {
				continue
			}
			metrics.GetMetrics().IncrementMetricsCounterWithLabels(metrics.GRPCConnectionLostCount, grpcEndpoint)
			log.Warn().Str("state", state.String()).Msg("gRPC Connection lost, attempting to reconnect...")
			if err := attemptReconnection(ctx, grpcConnection, &retryCount, &backoff, bc, grpcEndpoint); err != nil {
				return // Monitor shutdown due to max retries or context cancellation
			}
		}
	}
}

func attemptReconnection(
	ctx context.Context,
	conn *grpc.ClientConn,
	retryCount *int,
	backoff *time.Duration,
	bc backoffConfig,
	endpoint string,
) error {
	// Force reconnection attempt
	conn.ResetConnectBackoff()
	conn.Connect()

	if conn.GetState() == connectivity.Ready {
		log.Info().Msg("gRPC connection restored")
		metrics.GetMetrics().IncrementMetricsCounterWithLabels(metrics.GRPCReconnectionCount, endpoint)
		*retryCount = 0
		*backoff = bc.initial
		return nil
	}

	*retryCount++
	log.Warn().
		Int("retry", *retryCount).
		Dur("backoff", *backoff).
		Msg("Reconnection attempt failed")

	if *retryCount >= bc.maxRetries {
		metrics.GetMetrics().IncrementMetricsCounterWithLabels(metrics.GRPCConnectionPermanentFailure, endpoint)
		log.Error().Msg("Max reconnection attempts reached, triggering shutdown")
		return fmt.Errorf("max reconnection retries exceeded")
	}

	// Wait for backoff duration or context cancellation
	timer := time.NewTimer(*backoff)
	defer timer.Stop()

	select {
	case <-timer.C:
		*backoff = bc.nextBackoff(*backoff)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
	}
}

// InitializeGRPCClient initializes a gRPC client for the given endpoint with proper connection monitoring.
//
// Returns:
//   - the gRPC client connection
//   - a cancel function that stops the per-connection monitor goroutine.
//     Callers MUST invoke this cancel before discarding the returned conn
//     (typically right before grpcConnection.Close()) to avoid leaking the
//     monitor goroutine. If the parent ctx is cancelled the monitor will
//     also exit naturally, so passing a context.Background()-derived parent
//     here is only safe when the caller manages cancel explicitly.
func InitializeGRPCClient(ctx context.Context, grpcEndpoint string, insecureFlag bool) (*grpc.ClientConn, context.CancelFunc, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(8*1024*1024),
			grpc.MaxCallSendMsgSize(8*1024*1024),
			grpc.ForceCodec(&customCodec{}),
		),
	}

	// Configure transport security
	if insecureFlag {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := credentials.NewClientTLSFromCert(nil, "")
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	}

	log.Debug().
		Interface("dialOptions", dialOptions).
		Str("target", grpcEndpoint).
		Msg("Initializing gRPC client with options")

	log.Info().Msg("Creating new gRPC client")
	grpcConnection, err := grpc.NewClient(
		grpcEndpoint,
		dialOptions...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to %s: %w", grpcEndpoint, err)
	}

	// Per-connection child context so the caller can cancel JUST this monitor
	// (e.g. when forcing a reconnect that replaces the conn) without taking
	// down the whole process.
	monitorCtx, cancelMonitor := context.WithCancel(ctx)

	// Start connection monitoring in a separate goroutine
	go monitorGRPCConnection(monitorCtx, grpcConnection, grpcEndpoint)

	return grpcConnection, cancelMonitor, nil
}

// An interceptor that logs the gRPC request, for debugging purposes
// func loggerHeaderInterceptor() grpc.UnaryClientInterceptor {
// 	return func(
// 		ctx context.Context,
// 		method string,
// 		req, reply interface{},
// 		cc *grpc.ClientConn,
// 		invoker grpc.UnaryInvoker,
// 		opts ...grpc.CallOption,
// 	) error {
// 		log.Info().Str("method", method).Msg("Intercepting gRPC request")
// 		return invoker(ctx, method, req, reply, cc, opts...)
// 	}
// }
