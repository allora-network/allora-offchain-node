package lib

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"

	emissionstypes "github.com/allora-network/allora-chain/x/emissions/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/ignite/cli/v28/ignite/pkg/cosmosaccount"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
)

func getAlloraClient(config *UserConfig) (*cosmosclient.Client, error) {
	// create a allora client instance
	ctx := context.Background()
	userHomeDir, _ := os.UserHomeDir()
	alloraClientHome := filepath.Join(userHomeDir, ".allorad")
	if config.Wallet.AlloraHomeDir != "" {
		alloraClientHome = config.Wallet.AlloraHomeDir
	}

	// Check that the given home folder exist
	if _, err := os.Stat(alloraClientHome); errors.Is(err, os.ErrNotExist) {
		log.Println("Home directory does not exist, creating...")
		err = os.MkdirAll(alloraClientHome, 0755)
		if err != nil {
			log.Printf("Cannot create allora client home directory: %s. Error: %s", alloraClientHome, err)
			config.Wallet.SubmitTx = false
			return nil, err
		}
		log.Printf("Allora client home directory created: %s", alloraClientHome)
	}

	client, err := cosmosclient.New(ctx,
		cosmosclient.WithNodeAddress(config.Wallet.NodeRpc),
		cosmosclient.WithAddressPrefix(ADDRESS_PREFIX),
		cosmosclient.WithHome(alloraClientHome),
		cosmosclient.WithGas(config.Wallet.Gas),
		cosmosclient.WithGasAdjustment(config.Wallet.GasAdjustment),
	)
	if err != nil {
		log.Printf("Unable to create an allora blockchain client: %s", err)
		config.Wallet.SubmitTx = false
		return nil, err
	}
	return &client, nil
}

func (config *UserConfig) GenerateNodeConfig() (*NodeConfig, error) {
	config.Wallet.SubmitTx = true
	client, err := getAlloraClient(config)
	if err != nil {
		config.Wallet.SubmitTx = false
		return nil, err
	}
	var account cosmosaccount.Account
	// if we're giving a keyring ring name, with no mnemonic restore
	if config.Wallet.AddressRestoreMnemonic == "" && config.Wallet.AddressKeyName != "" {
		// get account from the keyring
		account, err = client.Account(config.Wallet.AddressKeyName)
		if err != nil {
			config.Wallet.SubmitTx = false
			log.Printf("could not retrieve account from keyring: %s", err)
		}
	} else if config.Wallet.AddressRestoreMnemonic != "" && config.Wallet.AddressKeyName != "" {
		// restore from mnemonic
		account, err = client.AccountRegistry.Import(config.Wallet.AddressKeyName, config.Wallet.AddressRestoreMnemonic, config.Wallet.AddressAccountPassphrase)
		if err != nil {
			if err.Error() == "account already exists" {
				account, err = client.Account(config.Wallet.AddressKeyName)
			}

			if err != nil {
				config.Wallet.SubmitTx = false
				log.Printf("could not restore account from mnemonic: %s", err)
			}
		}
	} else {
		log.Println("no allora account was loaded")
		return nil, nil
	}

	address, err := account.Address(ADDRESS_PREFIX)
	if err != nil {
		config.Wallet.SubmitTx = false
		log.Println("could not retrieve allora blockchain address, transactions will not be submitted to chain")
	} else {
		log.Printf("allora blockchain address loaded: %s", address)
	}

	// Create query client
	queryClient := emissionstypes.NewQueryClient(client.Context())

	// Create bank client
	bankClient := banktypes.NewQueryClient(client.Context())

	// this is terrible, no isConnected as part of this code path
	if client.Context().ChainID == "" {
		return nil, nil
	}

	config.Wallet.Address = address // Overwrite the address with the one from the keystore

	alloraChain := ChainConfig{
		Address:              address,
		AddressPrefix:        ADDRESS_PREFIX,
		DefaultBondDenom:     DEFAULT_BOND_DENOM,
		Account:              account,
		Client:               client,
		EmissionsQueryClient: queryClient,
		BankQueryClient:      bankClient,
	}

	Node := NodeConfig{
		Chain:   alloraChain,
		Wallet:  config.Wallet,
		Worker:  config.Worker,
		Reputer: config.Reputer,
	}

	return &Node, nil
}
