package lib

import (
	"allora_offchain_node/lib/auth"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	crypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/rs/zerolog/log"
)

type Wallet struct {
	Address          string
	AddressSDK       sdktypes.Address
	AccountNumber    uint64
	sequence         uint64
	AddressPrefix    string
	DefaultBondDenom string
	PubKey           cryptotypes.PubKey
	PrivKey          cryptotypes.PrivKey
	Keyring          keyring.Keyring

	mu sync.RWMutex
}

// NewWallet creates a new wallet instance
func NewWallet(
	address string,
	addressSDK sdktypes.Address,
	accountNumber uint64,
	sequence uint64,
	addressPrefix string,
	defaultBondDenom string,
	pubKey cryptotypes.PubKey,
	privKey cryptotypes.PrivKey,
	keyring keyring.Keyring,
) *Wallet {
	return &Wallet{ // nolint:exhaustruct
		Address:          address,
		AddressSDK:       addressSDK,
		AccountNumber:    accountNumber,
		sequence:         sequence,
		AddressPrefix:    addressPrefix,
		DefaultBondDenom: defaultBondDenom,
		PubKey:           pubKey,
		PrivKey:          privKey,
		Keyring:          keyring,
	}
}

// GetSequence returns the current sequence number
func (w *Wallet) GetSequence() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sequence
}

// SetSequence updates the sequence number
func (w *Wallet) SetSequence(sequence uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sequence = sequence
}

// IncrementSequence increments and returns the new sequence number
func (w *Wallet) IncrementSequence() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sequence++
	return w.sequence
}

// Creates a new wallet, partially filled with the wallet config
func NewWalletFromConfig(ctx context.Context, walletConfig WalletConfig) (*Wallet, error) {
	// Get kr
	kr, err := GetKeyring(walletConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get keyring: %w", err)
	}

	// Store address information
	var privKey cryptotypes.PrivKey
	var pubKey cryptotypes.PubKey
	var address string
	var addressSDK sdktypes.AccAddress

	// If a mnemonic is provided, use it to derive keys.
	if walletConfig.AddressRestoreMnemonic != "" {
		privKey, pubKey, address, addressSDK, err = GetAddressAndKeys(walletConfig.AddressRestoreMnemonic, walletConfig.AddressKeyName)
		if err != nil {
			return nil, fmt.Errorf("failed to get address and keys from mnemonic: %w", err)
		}
	} else {
		// Otherwise, extract the keys from the keyring.
		// Look up the key in the keyring.
		_, err = kr.Key(walletConfig.AddressKeyName)
		if err != nil {
			return nil, fmt.Errorf("key %s not found in keyring: %w", walletConfig.AddressKeyName, err)
		}

		// Export the armored private key using the provided passphrase.
		armored, err := kr.ExportPrivKeyArmor(walletConfig.AddressKeyName, walletConfig.KeyringPassphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to export armored private key: %w", err)
		}

		// Unarmor and decrypt to obtain the private key.
		privKey, algo, err := crypto.UnarmorDecryptPrivKey(armored, walletConfig.KeyringPassphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key with algo %s: %w", algo, err)
		}

		// Derive the public key and address.
		pubKey = privKey.PubKey()
		addressSDK = sdktypes.AccAddress(pubKey.Address())
		address = addressSDK.String()
	}

	// Check if key already exists in keyring
	log.Info().Msgf("Creating keyring for key %s on backend %s", walletConfig.AddressKeyName, walletConfig.KeyringBackend)
	_, err = kr.Key(walletConfig.AddressKeyName)
	if err == nil {
		log.Info().Msgf("Key %s already exists in keyring, skipping import", walletConfig.AddressKeyName)
	} else if !errors.Is(err, sdkerrors.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed to check keyring: %w", err)
	} else {
		log.Info().Msgf("Creating new key for key %s on backend %s, coin type %d", walletConfig.AddressKeyName, walletConfig.KeyringBackend, sdktypes.GetConfig().GetCoinType())
		_, err = kr.NewAccount(
			walletConfig.AddressKeyName,
			walletConfig.AddressRestoreMnemonic,
			walletConfig.KeyringPassphrase,
			hd.CreateHDPath(sdktypes.GetConfig().GetCoinType(), 0, 0).String(),
			hd.Secp256k1,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create key from mnemonic: %w", err)
		}
	}

	wallet := &Wallet{ // nolint: exhaustruct
		Keyring:          kr,
		Address:          address,
		AddressSDK:       addressSDK,
		PrivKey:          privKey,
		PubKey:           pubKey,
		AddressPrefix:    ADDRESS_PREFIX,
		DefaultBondDenom: DEFAULT_BOND_DENOM,
	}

	log.Info().Msgf("Wallet created successfully, %v", wallet)

	return wallet, nil
}

// GetKeyring initializes and returns a keyring instance.
// It creates the keyring directory if it doesn't exist.
func GetKeyring(walletConfig WalletConfig) (kr keyring.Keyring, err error) { // Remove pointer, keyring is already an interface
	// Determine home directory path
	var alloraClientHome string
	if walletConfig.AlloraHomeDir == "" {
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		alloraClientHome = filepath.Join(userHomeDir, ".allorad")
	} else {
		alloraClientHome = walletConfig.AlloraHomeDir
	}
	// Initialize keyring
	kr, err = keyring.New(
		"allora",
		walletConfig.KeyringBackend,
		alloraClientHome,
		os.Stdin,
		auth.GetKeyringCodec(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize backend %s keyring: %w", walletConfig.KeyringBackend, err)
	} else {
		log.Info().Msgf("Keyring backend %s initialized successfully on %s", walletConfig.KeyringBackend, alloraClientHome)
	}

	return kr, nil
}

// GetAddressAndKeys returns the private key, public key, and addresses from a mnemonic and key name
func GetAddressAndKeys(mnemonic string, keyName string) (cryptotypes.PrivKey, cryptotypes.PubKey, string, sdktypes.AccAddress, error) {
	if mnemonic == "" || keyName == "" {
		return nil, nil, "", nil, errors.New("mnemonic and key name are required")
	}

	// Get keys from mnemonic
	privKey, pubKey, address := GetPrivKey(ADDRESS_PREFIX, []byte(mnemonic))
	if privKey == nil || pubKey == nil || address == "" {
		return nil, nil, "", nil, errors.New("failed to generate keys from mnemonic")
	}

	// Convert to SDK address
	addressSDK, err := sdktypes.AccAddressFromBech32(address)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("failed to convert address to SDK address: %w", err)
	}

	return privKey, pubKey, address, addressSDK, nil
}

// Gets the private key, public key and address from a mnemonic
func GetPrivKey(prefix string, mnemonic []byte) (privKey cryptotypes.PrivKey, pubKey cryptotypes.PubKey, address string) {
	algo := hd.Secp256k1

	hdPath := fmt.Sprintf("m/44'/%d'/0'/0/%d", 118, 0)
	derivedPriv, err := algo.Derive()(string(mnemonic), "", hdPath)
	if err != nil {
		panic(err)
	}

	privKey = algo.Generate()(derivedPriv)
	pubKey = privKey.PubKey()

	addressbytes := sdktypes.AccAddress(pubKey.Address().Bytes())
	address, err = sdktypes.Bech32ifyAddressBytes(prefix, addressbytes)
	if err != nil {
		panic(err)
	}

	return privKey, pubKey, address
}
