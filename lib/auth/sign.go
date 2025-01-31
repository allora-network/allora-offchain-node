package auth

import (
	errorsmod "cosmossdk.io/errors"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/rs/zerolog/log"

	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// MarshallAndSignByPrivKey is a helper function to sign a message with a private key
func MarshallAndSignByPrivKey(payload proto.Message, privKey cryptotypes.PrivKey, address sdktypes.Address) (sig, pk []byte, err error) {
	protoBytesIn, err := proto.Marshal(payload)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error marshalling workerPayload") // nolint: exhaustruct
	}
	sig, err = privKey.Sign(protoBytesIn)
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "error signing the InferenceForecastsBundle message")
	}
	return sig, privKey.PubKey().Bytes(), nil
}

// MarshallAndSignByKeyring is a helper function to sign a message with a keyring
func MarshallAndSignByKeyring(payload proto.Message, keyring keyring.Keyring, address sdktypes.Address) (sig, pk []byte, err error) {
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

// MarshalProtoMessage dynamically marshals anytype of Protobuf message.
// Attempts to use XXX_Marshal if it exists, otherwise falls back to the default proto.Marshal.
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
