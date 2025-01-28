package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	cmtservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	feemarkettypes "github.com/skip-mev/feemarket/x/feemarket/types"

	errorsmod "cosmossdk.io/errors"
	cometrpc "github.com/cometbft/cometbft/rpc/client/http"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/keepalive"
)

// Used
// var cdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

func getAlloraRPCClient(config *UserConfig, rpc string) (rpcClient *cometrpc.HTTP, err error) {
	cmtCli, err := cometrpc.New(rpc, "/websocket")
	if err != nil {
		return nil, err
	}

	return cmtCli, nil
	// create a allora client instance
	// ctx := context.Background()
	// userHomeDir, _ := os.UserHomeDir()
	// alloraClientHome := filepath.Join(userHomeDir, ".allorad")
	// if config.Wallet.AlloraHomeDir != "" {
	// 	alloraClientHome = config.Wallet.AlloraHomeDir
	// }

	// // Check that the given home folder exists
	// if _, err := os.Stat(alloraClientHome); errors.Is(err, os.ErrNotExist) {
	// 	log.Info().Msg("Home directory does not exist, creating...")
	// 	err = os.MkdirAll(alloraClientHome, 0755)
	// 	if err != nil {
	// 		return nil, errorsmod.Wrap(err, "cannot create allora client home directory")
	// 	}
	// 	log.Info().Str("home", alloraClientHome).Msg("Allora client home directory created")
	// }

	// httpClient, err := jsonrpc.DefaultHTTPClient(rpc)
	// if err != nil {
	// 	return nil, fmt.Errorf("error creating default http client")
	// }

	// httpClient.Timeout = time.Duration(config.Wallet.TimeoutHTTPConnection) * time.Second
	// if transport, ok := httpClient.Transport.(*http.Transport); ok {
	// 	transport.DisableKeepAlives = false
	// 	transport.DisableCompression = false
	// 	transport.ForceAttemptHTTP2 = true
	// 	transport.MaxIdleConns = 100
	// 	transport.IdleConnTimeout = 90 * time.Second
	// 	transport.TLSHandshakeTimeout = 10 * time.Second
	// 	transport.ExpectContinueTimeout = 1 * time.Second
	// } else {
	// 	return nil, fmt.Errorf("unexpected transport type: %T", httpClient.Transport)
	// }

	// rpcClient, err := rpchttp.NewWithClient(rpc, "/websocket", httpClient)
	// if err != nil {
	// 	return nil, fmt.Errorf("error creating rpc client")
	// }

	// client, err := cosmosclient.New(ctx,
	// 	cosmosclient.WithNodeAddress(rpc),
	// 	cosmosclient.WithAddressPrefix(ADDRESS_PREFIX),
	// 	cosmosclient.WithHome(alloraClientHome),
	// 	cosmosclient.WithGas(config.Wallet.Gas),
	// 	cosmosclient.WithGasAdjustment(config.Wallet.GasAdjustment),
	// 	cosmosclient.WithAccountRetriever(authtypes.AccountRetriever{}),
	// 	cosmosclient.WithRPCClient(rpcClient),
	// )
	// if err != nil {
	// 	return nil, err
	// }
	// return &client, nil
}

func (chain *ChainConfig) headerInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		log.Info().Str("method", method).Msg("Intercepting gRPC request")
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// Initializes a gRPC client for the given endpoint
func (chain *ChainConfig) InitializeGRPCClient(grpcEndpoint string) (grpcConnection *grpc.ClientConn, err error) {
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
			grpc.MaxCallRecvMsgSize(128*1024*1024),
			grpc.MaxCallSendMsgSize(128*1024*1024),
			grpc.ForceCodec(customCodec),
		),
	)
	creds := credentials.NewClientTLSFromCert(nil, "")
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	// Add interceptor for logging if needed
	// dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(chain.headerInterceptor()))
	log.Debug().Interface("dialOptions", dialOptions).Str("target", grpcEndpoint).Msg("Dial options")
	log.Info().Msg("Creating new gRPC client")
	grpcConnection, err = grpc.NewClient(
		grpcEndpoint,
		dialOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", grpcEndpoint, err)
	}

	// spin up goroutine for monitoring and reconnect purposes
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

func (c *UserConfig) GenerateNodeConfig(ctx context.Context, wallet *Wallet, rpc string, grpc string) (nodeConfig *NodeConfig, err error) {

	// // Get keyring
	// keyring, err := GetKeyring(c.Wallet.AlloraHomeDir)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get keyring: %w", err)
	// }

	// // Store address information
	// var privKey cryptotypes.PrivKey
	// var pubKey cryptotypes.PubKey
	// var address string

	// privKey, pubKey, address, addressSDK, err := GetAddressAndKeys(c.Wallet.AddressRestoreMnemonic, c.Wallet.AddressKeyName)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get address and keys: %w", err)
	// }

	c.Wallet.Address = wallet.Address // Overwrite the address with the one from the keystore

	log.Info().Str("rpc", rpc).Str("address", wallet.Address).Msg("Allora client created successfully")

	// TODO: This is a temporary solution to get the chain config working, will be removed after refactor
	alloraChain := ChainConfig{ // nolint: exhaustruct
		Keyring:          wallet.Keyring,
		Address:          wallet.Address,
		AddressSDK:       wallet.AddressSDK,
		PrivKey:          wallet.PrivKey,
		PubKey:           wallet.PubKey,
		AddressPrefix:    ADDRESS_PREFIX,
		DefaultBondDenom: DEFAULT_BOND_DENOM,
		Client:           &Client{}, // nolint: exhaustruct
	}

	Node := NodeConfig{
		ServerAddress: rpc,
		Chain:         alloraChain,
		Wallet:        c.Wallet,
		Worker:        c.Worker,
		Reputer:       c.Reputer,
	}

	// Get RPC allora client
	var rpcClient *cometrpc.HTTP
	if rpc != "" {
		rpcClient, err = getAlloraRPCClient(c, rpc)
		if err != nil {
			return nil, err
		}
		Node.Chain.Client.RPCClient = rpcClient
		Node.ServerAddress = rpc
		log.Info().Msgf("RPC Node initialized successfully, with account (sequence: %d, accNum: %d)", wallet.GetSequence(), wallet.AccountNumber)
	}

	// Get GRPC allora client
	if grpc != "" {
		grpcConn, err := alloraChain.InitializeGRPCClient(grpc)
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to initialize gRPC client")
		}
		Node.Chain.Client.GRPCClient = grpcConn
		// Create query client
		Node.Chain.EmissionsQueryClient = emissionstypes.NewQueryServiceClient(grpcConn)

		Node.Chain.BankQueryClient = banktypes.NewQueryClient(grpcConn)
		Node.Chain.AuthQueryClient = authtypes.NewQueryClient(grpcConn)
		Node.Chain.FeeMarketQueryClient = feemarkettypes.NewQueryClient(grpcConn)
		Node.Chain.CometQueryClient = cmtservice.NewServiceClient(grpcConn)
		Node.Chain.Client.GRPCClient = grpcConn
		Node.ServerAddress = grpc

		if wallet.GetSequence() == 0 {
			_, sequence, accNum, err := Node.GetAccountInfo(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get account info: %w", err)
			}
			wallet.SetSequence(sequence)
			wallet.AccountNumber = accNum
			log.Info().Msgf("GRPC Node initialized successfully, with account (sequence: %d, accNum: %d)", sequence, accNum)
		} else {
			log.Info().Msgf("GRPC Node initialized successfully, with account (sequence: %d, accNum: %d)", wallet.GetSequence(), wallet.AccountNumber)
		}

	}
	// TODO Remove Legacy "Chain" object
	Node.Chain.AccNum = wallet.AccountNumber
	Node.Chain.Sequence = wallet.GetSequence()
	return &Node, nil
}
