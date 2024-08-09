package lib

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

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
	if config.Wallets.AlloraHomeDir != "" {
		alloraClientHome = config.Wallets.AlloraHomeDir
	}

	// Check that the given home folder exist
	if _, err := os.Stat(alloraClientHome); errors.Is(err, os.ErrNotExist) {
		log.Info().Msg("Home directory does not exist, creating...")
		err = os.MkdirAll(alloraClientHome, 0755)
		if err != nil {
			log.Error().Err(err).Str("home", alloraClientHome).Msg("Cannot create allora client home directory")
			config.Wallets.SubmitTx = false
			return nil, err
		}
		log.Info().Str("home", alloraClientHome).Msg("Allora client home directory created")
	}

	client, err := cosmosclient.New(ctx,
		cosmosclient.WithNodeAddress(config.Wallets.NodeRpc),
		cosmosclient.WithAddressPrefix(ADDRESS_PREFIX),
		cosmosclient.WithHome(alloraClientHome),
		cosmosclient.WithGas(config.Wallets.Gas),
		cosmosclient.WithGasAdjustment(config.Wallets.GasAdjustment),
	)
	if err != nil {
		log.Error().Err(err).Msg("Unable to create an allora blockchain client")
		config.Wallets.SubmitTx = false
		return nil, err
	}
	return &client, nil
}

func configWallet(client *cosmosclient.Client, walletConfig WalletConfig) error {
	walletConfig.SubmitTx = true

	var account cosmosaccount.Account
	var err error
	// if we're giving a keyring ring name, with no mnemonic restore
	if walletConfig.AddressRestoreMnemonic == "" && walletConfig.AddressKeyName != "" {
		// get account from the keyring
		account, err = client.Account(walletConfig.AddressKeyName)
		if err != nil {
			walletConfig.SubmitTx = false
			log.Error().Err(err).Msg("could not retrieve account from keyring")
			return err
		}
	} else if walletConfig.AddressRestoreMnemonic != "" && walletConfig.AddressKeyName != "" {
		// restore from mnemonic
		account, err = client.AccountRegistry.Import(walletConfig.AddressKeyName, walletConfig.AddressRestoreMnemonic, "")
		if err != nil {
			if err.Error() == "account already exists" {
				account, err = client.Account(walletConfig.AddressKeyName)
			}

			if err != nil {
				walletConfig.SubmitTx = false
				log.Err(err).Msg("could not restore account from mnemonic")
				return err
			}
		}
	} else {
		log.Debug().Msg("no allora account was loaded")
		return errors.New("no allora account was loaded")
	}

	address, err := account.Address(ADDRESS_PREFIX)
	if err != nil {
		walletConfig.SubmitTx = false
		log.Err(err).Msg("could not retrieve allora blockchain address, transactions will not be submitted to chain")
		return err
	}
	walletConfig.Address = address // Overwrite the address with the one from the keystore
	log.Info().Str("Account", walletConfig.AddressKeyName).Str("address", address).Msg("Allora address loaded succesfully")

	return nil
}

func (config *UserConfig) GenerateNodeConfig() (*NodeConfig, error) {
	// Use one alloraClient for all wallets
	client, err := getAlloraClient(config)
	if err != nil {
		return nil, err
	}

	// TODO Modify to iterate through wallets
	for _, wallet := range config.Wallets {
		err := configWallet(client, wallet)
		if err != nil {
			return nil, err
		}
	}

	// Create query client
	queryClient := emissionstypes.NewQueryClient(client.Context())

	// Create bank client
	bankClient := banktypes.NewQueryClient(client.Context())

	// this is terrible, no isConnected as part of this code path
	if client.Context().ChainID == "" {
		return nil, nil
	}

	alloraChain := ChainConfig{
		AddressPrefix:        ADDRESS_PREFIX,
		DefaultBondDenom:     DEFAULT_BOND_DENOM,
		Client:               client,
		EmissionsQueryClient: queryClient,
		BankQueryClient:      bankClient,
	}

	Node := NodeConfig{
		Chain:   alloraChain,
		Wallets: config.Wallets,
		Worker:  config.Worker,
		Reputer: config.Reputer,
	}

	log.Info().Msg("Allora client created successfully")
	return &Node, nil
}
