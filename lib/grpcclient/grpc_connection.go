package grpcclient

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/keepalive"
)

// Initializes a gRPC client for the given endpoint
func InitializeGRPCClient(grpcEndpoint string) (grpcConnection *grpc.ClientConn, err error) {
	var dialOptions []grpc.DialOption

	kaOpts := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}

	customCodec := &customCodec{parentCodec: encoding.GetCodec("proto")}
	dialOptions = append(dialOptions, grpc.WithKeepaliveParams(kaOpts))
	dialOptions = append(dialOptions,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(8*1024*1024),
			grpc.MaxCallSendMsgSize(8*1024*1024),
			grpc.ForceCodec(customCodec),
		),
	)
	creds := credentials.NewClientTLSFromCert(nil, "")
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
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

	// TODO: Investigate and ecide if we want to keep this
	// spin up goroutine for monitoring and reconnect purposes - TODO test and configure
	// go func() {
	// 	for {
	// 		state := chain.Client.GRPCClient.GetState()
	// 		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
	// 			fmt.Println("GRPC Connection lost, attempting to reconnect...")
	// 			for {
	// 				if chain.Client.GRPCClient.WaitForStateChange(context.Background(), state) {
	// 					break
	// 				}
	// 				time.Sleep(10 * time.Second)
	// 			}
	// 		}
	// 		time.Sleep(10 * time.Second)
	// 	}
	// }()
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
