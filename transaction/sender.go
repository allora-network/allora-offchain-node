package transaction

import (
	"context"
	"fmt"

	"allora_offchain_node/client"
	"allora_offchain_node/types"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
)

var cdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

// SendTransactionViaRPC sends a transaction using the provided rpc, ransactionParams and sequence number.
func SendTransactionViaRPC(ctx context.Context, rpcEndpoint string, txParams *types.TransactionParams, sequence uint64, waitForTx bool, msgs ...sdktypes.Msg) (*coretypes.ResultBroadcastTx, string, error) {
	// TODO use non-test encoding config
	encodingConfig := moduletestutil.MakeTestEncodingConfig()
	encodingConfig.Codec = cdc

	// Build and sign the transaction
	txBytes, err := BuildAndSignTransaction(ctx, txParams, sequence, encodingConfig, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := Transaction(ctx, txBytes, rpcEndpoint, waitForTx)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}

// Transaction broadcasts the transaction bytes to the given RPC endpoint.
func Transaction(ctx context.Context, txBytes []byte, rpcEndpoint string, waitForTx bool) (*coretypes.ResultBroadcastTx, error) {
	client, err := client.GetClient(rpcEndpoint)
	if err != nil {
		return nil, err
	}
	resp, err := client.BroadcastTx(ctx, txBytes, waitForTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, nil
}
