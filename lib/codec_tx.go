package lib

import (
	"cosmossdk.io/x/tx/signing"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
)

var (
	txConfig sdkclient.TxConfig
)

func init() {
	var err error
	txConfig, err = DefaultTxConfig()
	if err != nil {
		panic(err)
	}
}

func DefaultTxConfig() (sdkclient.TxConfig, error) {
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
			},
			ValidatorAddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return authtx.NewTxConfig(
		codec.NewProtoCodec(interfaceRegistry),
		authtx.DefaultSignModes,
	), nil
}
