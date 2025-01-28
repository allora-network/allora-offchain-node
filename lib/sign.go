package lib

import (
	errorsmod "cosmossdk.io/errors"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/rs/zerolog/log"

	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

func MarshallAndSignByPrivKey(payload proto.Message, privKey cryptotypes.PrivKey, address sdktypes.Address) (sig, pk []byte, err error) {
	protoBytesIn := make([]byte, 0)
	protoBytesIn, err = proto.Marshal(payload)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error marshalling workerPayload") // nolint: exhaustruct
	}
	sig, err = privKey.Sign(protoBytesIn)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error signing the InferenceForecastsBundle message")
	}
	return sig, privKey.PubKey().Bytes(), nil
}

func MarshallAndSignByKeyring(payload proto.Message, keyring keyring.Keyring, address sdktypes.Address) (sig, pk []byte, err error) {
	// protoBytesIn := make([]byte, 0)
	// protoBytesIn, err = payload.XXX_Marshal(protoBytesIn, true)
	// if err != nil {
	// 	return nil, nil, errorsmod.Wrapf(err, "error marshalling workerPayload") // nolint: exhaustruct
	// }
	protoBytesIn, err := MarshalProtoMessage(payload)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error marshalling workerPayload") // nolint: exhaustruct
	}
	var pubKey cryptotypes.PubKey
	sig, pubKey, err = keyring.SignByAddress(
		address,
		protoBytesIn,
		signing.SignMode_SIGN_MODE_DIRECT)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error signing the InferenceForecastsBundle message") // nolint: exhaustruct
	}
	return sig, pubKey.Bytes(), nil
}

// MarshalProtoMessage dynamically marshals the Protobuf message.
func MarshalProtoMessage(msg proto.Message) ([]byte, error) {
	// Check if XXX_Marshal exists on the type.
	if m, ok := msg.(interface {
		XXX_Marshal([]byte, bool) ([]byte, error)
	}); ok {
		log.Info().Msgf("USING XXX_MARSHAL")
		return m.XXX_Marshal([]byte{}, true)
	}

	log.Info().Msgf("USING DEFAULT MARSHAL")
	// Fallback to the default proto.Marshal if XXX_Marshal doesn't exist
	return proto.Marshal(msg)
}
