package main

import (
	"allora_offchain_node/lib"
	// reputerCoinGecko "allora_offchain_node/pkg/reputer_coingecko_l1_norm"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
	"os"
)

var UserConfig = lib.UserConfig{
	Wallet: lib.WalletConfig{
		AddressKeyName:           os.Getenv("ALLORA_ACCOUNT_NAME"),       // load a address by key from the keystore testing = allo1wmfp4xuurjsvh3qzjhkdxqgepmshpy7ny88pc7
		AddressRestoreMnemonic:   os.Getenv("ALLORA_ACCOUNT_MNEMONIC"),   // mnemonic for the allora account
		AddressAccountPassphrase: os.Getenv("ALLORA_ACCOUNT_PASSPHRASE"), // passphrase for the allora account
		AlloraHomeDir:            "",                                     // home directory for the allora keystore, if "", it will automatically create in "$HOME/.allorad"
		Gas:                      "1000000",                              // gas to use for the allora client in uallo
		GasAdjustment:            1.0,                                    // gas adjustment to use for the allora client
		SubmitTx:                 true,                                   // set to false to run in dry-run processes without committing to the chain. useful for development and testing
		NodeRpc:                  os.Getenv("ALLORA_NODE_RPC"),
		MaxRetries:               3,
		MinDelay:                 1,
		MaxDelay:                 6,
		EarlyArrivalPercent:      60,
		LateArrivalPercent:       10,
	},
	Worker: []lib.WorkerConfig{
		{
			TopicId:             1,
			InferenceEntrypoint: worker10min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  nil,
			LoopSeconds:         5,
			ExtraData: map[string]string{
				"inferenceEndpoint": "http://localhost:8000/inference",
				"token":             "ETH",
				"forecastEndpoint":  "http://localhost:8000/forecast",
			},
		},
	},
	Reputer: []lib.ReputerConfig{
		{
			TopicId:           1,
			ReputerEntrypoint: worker10min.NewAlloraEntrypoint(),
			LoopSeconds:       30,
			MinStake:          100000,
		},
	},
}
