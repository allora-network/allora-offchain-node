package lib

import (
	"context"
	"errors"

	cosmossdk_io_math "cosmossdk.io/math"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (node *NodeConfig) GetBalance(accountName string) (cosmossdk_io_math.Int, error) {
	ctx := context.Background()
	// Get wallet from account name
	wallet, exists := node.Wallets[accountName]
	if !exists {
		return cosmossdk_io_math.Int{}, errors.New("No wallet found for account name")
	}

	resp, err := node.Chain.BankQueryClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: wallet.Address,
		Denom:   node.Chain.DefaultBondDenom,
	})
	if err != nil {
		return cosmossdk_io_math.Int{}, err
	}
	return resp.Balance.Amount, nil
}
