package main

import (
	"allora_offchain_node/lib"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
)

var UserConfig = lib.UserConfig{
	Wallet: lib.WalletConfig{
		AddressKeyName: "offchain1", // load a address by key from the keystore
		// mnemonic for the allora account
		AddressRestoreMnemonic: "bottom wrong raise true school treat urge plug predict endless pipe around require repair knee bubble prison urban earth language midnight gun yard citizen",

		AddressAccountPassphrase: "",                    // passphrase for the allora account
		AlloraHomeDir:            "/home/diego/.allora", // home directory for the allora keystore
		Gas:                      "1000000",             // gas to use for the allora client in uallo
		GasAdjustment:            1.0,                   // gas adjustment to use for the allora client
		SubmitTx:                 true,                  // set to false to run in dry-run processes without committing to the chain. useful for development and testing
		LoopWithinWindowSeconds:  5,
		NodeRpc:                  "http://localhost:26657",
		MaxRetries:               3,
		MinDelay:                 1,
		MaxDelay:                 6,
		EarlyArrivalPercent:      0.6,
		LateArrivalPercent:       0.1,
	},
	Worker: []lib.WorkerConfig{
		{
			TopicId:             1,
			InferenceEntrypoint: worker10min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  nil,
			ExtraData: map[string]string{
				"inferenceEndpoint": "http://localhost:8000/inference",
				"token":             "ETH",
				"forecastEndpoint":  "http://localhost:8000/forecast",
			},
		},
		// {
		// 	TopicId:             2,
		// 	InferenceEntrypoint: worker20min.NewAlloraEntrypoint(),
		// 	ForecastEntrypoint:  worker20min.NewAlloraEntrypoint(),
		// },
	},
	// Reputer: []lib.ReputerConfig{
	// 	{
	// 		TopicId:           1,
	// 		ReputerEntrypoint: reputerCoinGecko.NewAlloraEntrypoint(),
	// 		MinStake:          1000000,
	// 	},
	// },
}
