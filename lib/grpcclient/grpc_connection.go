package grpcclient

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"allora_offchain_node/metrics"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func monitorGRPCConnection(ctx context.Context, grpcConnection *grpc.ClientConn, grpcEndpoint string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	maxRetries := 5
	retryCount := 0
	initialBackoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	backoff := initialBackoff

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Shutting down gRPC monitoring goroutine.")
			return
		case <-ticker.C:
			state := grpcConnection.GetState()
			if state == connectivity.TransientFailure || state == connectivity.Shutdown {
				log.Warn().Msg("gRPC Connection lost, attempting to reconnect...")

				// Force reconnection attempt
				grpcConnection.ResetConnectBackoff()
				grpcConnection.Connect()
				if grpcConnection.GetState() != connectivity.Ready {
					retryCount++
					log.Warn().Int("retry", retryCount).
						Dur("backoff", backoff).
						Msg("Reconnection attempt failed")

					if retryCount >= maxRetries {
						log.Error().Msg("Max reconnection attempts reached, triggering shutdown")
						return
					}

					// Exponential backoff with jitter
					jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
					time.Sleep(backoff + jitter)
					backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
				} else {
					log.Info().Msg("gRPC connection restored")
					metrics.GetMetrics().IncrementMetricsCounterWithLabels(metrics.GRPCConnectionLostCount, grpcEndpoint)
					retryCount = 0           // Reset counter on success
					backoff = initialBackoff // Reset backoff on success
				}
			}
		}
	}
}

// Initializes a gRPC client for the given endpoint
func InitializeGRPCClient(ctx context.Context, grpcEndpoint string, insecureFlag bool) (grpcConnection *grpc.ClientConn, err error) {
	var dialOptions []grpc.DialOption

	kaOpts := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}

	customCodec := &customCodec{}
	dialOptions = append(dialOptions, grpc.WithKeepaliveParams(kaOpts))
	dialOptions = append(dialOptions,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(8*1024*1024),
			grpc.MaxCallSendMsgSize(8*1024*1024),
			grpc.ForceCodec(customCodec),
		),
	)

	if insecureFlag {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := credentials.NewClientTLSFromCert(nil, "")
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	}

	// Add interceptor for logging if needed
	// dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(loggerHeaderInterceptor()))
	log.Debug().Interface("dialOptions", dialOptions).Str("target", grpcEndpoint).Msg("Dial options")
	log.Info().Msg("Creating new gRPC client")
	grpcConnection, err = grpc.NewClient(
		grpcEndpoint,
		dialOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", grpcEndpoint, err)
	}

	// spin up goroutine for monitoring and reconnect purposes - TODO test and configure
	go monitorGRPCConnection(ctx, grpcConnection, grpcEndpoint)

	return grpcConnection, nil
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
