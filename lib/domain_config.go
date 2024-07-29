package lib

import (
	"log"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosaccount"
	"github.com/ignite/cli/v28/ignite/pkg/cosmosclient"
)

// Properties manually provided by the user as part of UserConfig
type WalletConfig struct {
	Address                  string // will be overwritten by the keystore. This is the 1 value that is auto-generated in this struct
	AddressKeyName           string // load a address by key from the keystore
	AddressRestoreMnemonic   string
	AddressAccountPassphrase string
	AlloraHomeDir            string  // home directory for the allora keystore
	Gas                      string  // gas to use for the allora client
	GasAdjustment            float64 // gas adjustment to use for the allora client
	SubmitTx                 bool    // do we need to commit these to the chain, might be a reason not to
	LoopWithinWindowSeconds  int64   // how often to run the main loops per worker and per reputer
	NodeRpc                  string  // rpc node for allora chain
	MaxRetries               int64   // retry to get data from chain up to this many times per query or tx
	MinDelay                 int64   // minimum of uniform distribution that is sampled then used to calcluate exponential backoff for txs (in seconds)
	MaxDelay                 int64   // maximum of uniform distribution that is sampled then used to calcluate exponential backoff for txs (in seconds)
	// If a topic's next open nonce is predicted to be in 10 blocks by
	// `topic.EpochLastEnded + topic.EpochLength - now() == 10`, and this value is X,
	// then we'll actually start to query for the nonce in (1-X)*10 blocks.
	// This is useful because block time is variable => this ensures we don't miss the next open nonce.
	// The higher this is, the earlier we start querying for the nonce.
	EarlyArrivalPercent float64
	// If this value is Y, and the next open nonce is predicted to be 10 as in the example above,
	// then we'll end querying for the nonce in (1+Y)*10 blocks.
	// This is useful because block time is variable => this ensures we don't prematurely stop querying for an open nonce.
	LateArrivalPercent float64
}

// Properties auto-generated based on what the user has provided in WalletConfig fields of UserConfig
type ChainConfig struct {
	Address              string // will be auto-generated based on the keystore
	Account              cosmosaccount.Account
	Client               *cosmosclient.Client
	EmissionsQueryClient emissions.QueryClient
	BankQueryClient      bank.QueryClient
	DefaultBondDenom     string
	AddressPrefix        string // prefix for the allora addresses
}

type WorkerConfig struct {
	TopicId               emissions.TopicId
	InferenceEntrypoint   AlloraEntrypoint
	ForecastEntrypoint    AlloraEntrypoint
	AllowsNegativeForcast bool
}

type ReputerConfig struct {
	TopicId           emissions.TopicId
	ReputerEntrypoint AlloraEntrypoint
	// Minimum stake to repute. will try to add stake from wallet if current stake is less than this.
	// Will not repute if current stake is less than this, after trying to add any necessary stake.
	// This is idempotent in that it will not add more stake than specified here.
	// Set to 0 to effectively disable this feature and use whatever stake has already been added.
	MinStake int64
}

type UserConfig struct {
	Wallet  WalletConfig
	Worker  []WorkerConfig
	Reputer []ReputerConfig
}

type NodeConfig struct {
	Chain   ChainConfig
	Wallet  WalletConfig
	Worker  []WorkerConfig
	Reputer []ReputerConfig
}

type WorkerResponse struct {
	WorkerConfig
	InfererValue     string      `json:"infererValue,omitempty"`
	ForecasterValues []ForecastResponse `json:"forecasterValue,omitempty"`
}

type SignedWorkerResponse struct {
	*emissions.WorkerDataBundle
	BlockHeight int64 `json:"blockHeight,omitempty"`
	TopicId     int64 `json:"topicId,omitempty"`
}

// Check that each assigned entrypoint in `TheConfig` actually can be used
// for the intended purpose, else throw error
func (c *UserConfig) ValidateConfigEntrypoints() {
	for _, workerConfig := range c.Worker {
		if workerConfig.InferenceEntrypoint != nil && !workerConfig.InferenceEntrypoint.CanInfer() {
			log.Fatal("Invalid inference entrypoint: ", workerConfig.InferenceEntrypoint)
		}
		if workerConfig.ForecastEntrypoint != nil && !workerConfig.ForecastEntrypoint.CanForecast() {
			log.Fatal("Invalid forecast entrypoint: ", workerConfig.ForecastEntrypoint)
		}
	}

	for _, reputerConfig := range c.Reputer {
		if reputerConfig.ReputerEntrypoint != nil && !reputerConfig.ReputerEntrypoint.CanSourceTruth() {
			log.Fatal("Invalid loss entrypoint: ", reputerConfig.ReputerEntrypoint)
		}
	}
}
