package main

import (
	"allora_offchain_node/lib"
	reputerCoinGecko "allora_offchain_node/pkg/reputer_coingecko_l1_norm"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
)

var UserConfig = lib.UserConfig{
	Wallet: lib.WalletConfig{
		AddressKeyName:           "test-offchain",      // load a address by key from the keystore testing = allo1wmfp4xuurjsvh3qzjhkdxqgepmshpy7ny88pc7
		AddressRestoreMnemonic:   "your mnemonic here", // mnemonic for the allora account
		AddressAccountPassphrase: "secret",             // passphrase for the allora account
		AlloraHomeDir:            "",                   // home directory for the allora keystore, if "", it will automatically create in "$HOME/.allorad"
		Gas:                      "1000000",            // gas to use for the allora client in uallo
		GasAdjustment:            1.0,                  // gas adjustment to use for the allora client
		SubmitTx:                 true,                 // set to false to run in dry-run processes without committing to the chain. useful for development and testing
		NodeRpc:                  "http://localhost:26657",
		// NodeRpc: "https://allora-rpc.testnet-1.testnet.allora.network/",
		MaxRetries:          3,
		MinDelay:            1,
		MaxDelay:            6,
		EarlyArrivalPercent: 60,
		LateArrivalPercent:  10,
	},
	Worker: []lib.WorkerConfig{
		{
			TopicId:             1,
			InferenceEntrypoint: worker10min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  nil,
			LoopSeconds:         5,
		},
	},
	Reputer: []lib.ReputerConfig{
		{
			TopicId:           1,
			ReputerEntrypoint: reputerCoinGecko.NewAlloraEntrypoint(),
			LoopSeconds:       30,
			MinStake:          10,
		},
	},
}
