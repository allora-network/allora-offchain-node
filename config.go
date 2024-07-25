package main

import (
	reputerCoinGecko "allora_offchain_node/pkg/reputer_coingecko_l1_norm"
	worker10min "allora_offchain_node/pkg/worker_coin_predictor_10min_eth"
	worker20min "allora_offchain_node/pkg/worker_coin_predictor_20min"
	"allora_offchain_node/types"
)

var UserConfig = types.UserConfig{
	Wallet: types.WalletConfig{
		Address:                  "allo123",              // address for the allora account
		AddressKeyName:           "secret",               // load a address by key from the keystore
		AddressRestoreMnemonic:   "secret",               // mnemonic for the allora account
		AddressAccountPassphrase: "secret",               // passphrase for the allora account
		AlloraHomeDir:            "/home/allora/.allora", // home directory for the allora keystore
		Gas:                      "1000000",              // gas to use for the allora client in uallo
		GasAdjustment:            1.0,                    // gas adjustment to use for the allora client
		SubmitTx:                 true,                   // set to false to run in dry-run processes without committing to the chain. useful for development and testing
		NodeRpc:                  "http://rpc.allora.network",
		RequestRetries:           3,
		LoopSeconds:              60,
		MinStakeToRepute:         "100",
	},
	Worker: []types.WorkerConfig{
		{
			TopicId:             1,
			InferenceEntrypoint: worker10min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  nil,
		},
		{
			TopicId:             2,
			InferenceEntrypoint: worker20min.NewAlloraEntrypoint(),
			ForecastEntrypoint:  worker20min.NewAlloraEntrypoint(),
		},
	},
	Reputer: []types.ReputerConfig{
		{
			TopicId:           1,
			ReputerEntrypoint: reputerCoinGecko.NewAlloraEntrypoint(),
		},
	},
}
