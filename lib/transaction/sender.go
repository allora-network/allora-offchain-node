package transaction

import (
	"context"
	"fmt"

	types "allora_offchain_node/lib/types"

	"allora_offchain_node/lib/rpcclient"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// SendTransactionViaRPC sends a transaction using the provided rpc, TransactionParams and sequence number.
func SendTransactionViaRPC(ctx context.Context,
	rpcClient *rpcclient.AlloraRPCClient,
	rpcEndpoint string,
	txParams *types.TransactionParams,
	sequence uint64,
	waitForTx bool,
	msgs ...sdktypes.Msg,
) (*coretypes.ResultBroadcastTx, string, error) {

	// Build and sign the transaction
	encodingConfig := GetEncodingConfig()
	txBytes, err := BuildAndSignTransaction(ctx, txParams, sequence, encodingConfig, msgs...)
	if err != nil {
		return nil, "", err
	}

	// Broadcast the transaction via RPC
	resp, err := rpcClient.BroadcastTx(ctx, txBytes, waitForTx)
	if err != nil {
		return resp, string(txBytes), fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	return resp, string(txBytes), nil
}
