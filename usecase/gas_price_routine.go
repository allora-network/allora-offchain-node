package usecase

import (
	"context"
	"time"

	"allora_offchain_node/lib"

	"github.com/rs/zerolog/log"
)

// UpdateGasPriceRoutine continuously updates the gas price at a specified interval
func (suite *UseCaseSuite) UpdateGasPriceRoutine(ctx context.Context) {
	wallet, err := suite.RPCManager.GetWallet()
	if err != nil {
		log.Error().Err(err).Msg("Error getting wallet")
		return
	}
	walletConfig, err := suite.RPCManager.GetWalletConfig()
	if err != nil {
		log.Error().Err(err).Msg("Error getting wallet config")
		return
	}
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Updating fee price routine: terminating.")
			return
		default:
			price, err := lib.RunWithNodeRetry(
				ctx,
				suite.RPCManager,
				func(node *lib.NodeConfig) (float64, error) {
					return WithTimeoutResult(ctx,
						time.Duration(walletConfig.TimeoutRPCSecondsQuery)*time.Second,
						func(ctx context.Context) (float64, error) {
							return suite.RPCManager.GetCurrentQueryNode().GetBaseFee(ctx, wallet.DefaultBondDenom)
						})
				},
				"get base fee",
				lib.GRPC_MODE,
			)
			if err != nil {
				log.Error().Err(err).Msg("Error updating gas prices")
			}
			lib.SetGasPrice(price)
			log.Debug().Float64("gasPrice", lib.GetGasPrice()).Msg("Updating fee price routine: updating value.")
			time.Sleep(time.Duration(walletConfig.GasPriceUpdateInterval) * time.Second)
		}
	}
}
