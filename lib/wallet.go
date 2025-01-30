package lib

import (
	"allora_offchain_node/lib/auth"
	"allora_offchain_node/lib/rpcclient"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
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
	return &Wallet{
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
	// Get keyring
	keyring, err := GetKeyring(walletConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get keyring: %w", err)
	}

	// Store address information
	var privKey cryptotypes.PrivKey
	var pubKey cryptotypes.PubKey
	var address string

	privKey, pubKey, address, addressSDK, err := GetAddressAndKeys(walletConfig.AddressRestoreMnemonic, walletConfig.AddressKeyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get address and keys: %w", err)
	}

	keyring.ImportPrivKey(walletConfig.AddressKeyName, walletConfig.AddressRestoreMnemonic, "")

	wallet := &Wallet{ // nolint: exhaustruct
		Keyring:          keyring,
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
		keyring.BackendTest,
		alloraClientHome,
		os.Stdin,
		auth.GetKeyringCodec(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize keyring: %w", err)
	}

	return kr, nil
}

// GetAddressAndKeys returns the private key, public key, and addresses from a mnemonic and key name
func GetAddressAndKeys(mnemonic string, keyName string) (cryptotypes.PrivKey, cryptotypes.PubKey, string, sdktypes.AccAddress, error) {
	if mnemonic == "" || keyName == "" {
		return nil, nil, "", nil, errors.New("mnemonic and key name are required")
	}

	// Get keys from mnemonic
	privKey, pubKey, address := rpcclient.GetPrivKey(ADDRESS_PREFIX, []byte(mnemonic))
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
