package lib

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types/query"
	auth "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/rs/zerolog/log"
)

// GetBaseFee queries the current base fee from the feemarket module
func (node *NodeConfig) GetAccountInfo(ctx context.Context) (address string, sequence uint64, accNum uint64, err error) {
	log.Info().Msgf("Getting account info for %s", node.Chain.Address)
	resp, err := QueryDataWithRetry(
		ctx,
		node.Wallet.MaxRetries,
		node.Wallet.RetryDelay,
		func(ctx context.Context, req query.PageRequest) (*auth.QueryAccountInfoResponse, error) {
			return node.Chain.AuthQueryClient.AccountInfo(ctx, &auth.QueryAccountInfoRequest{Address: node.Chain.Address})
		},
		query.PageRequest{}, // nolint:exhaustruct
		"get account info",
		node,
	)
	if err != nil {
		return "", 0, 0, err
	}

	if resp.Info == nil {
		return "", 0, 0, fmt.Errorf("account info is nil")
	}

	address = resp.Info.Address
	sequence = resp.Info.Sequence
	accNum = resp.Info.AccountNumber

	return address, sequence, accNum, nil
}
