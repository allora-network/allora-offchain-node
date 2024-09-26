package lib

import (
	"context"

	cosmossdk_io_math "cosmossdk.io/math"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (node *NodeConfig) GetBalance(ctx context.Context) (cosmossdk_io_math.Int, error) {
	resp, err := node.Chain.BankQueryClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: node.Chain.Address,
		Denom:   node.Chain.DefaultBondDenom,
	})
	if err != nil {
		return cosmossdk_io_math.Int{}, err
	}
	return resp.Balance.Amount, nil
}
