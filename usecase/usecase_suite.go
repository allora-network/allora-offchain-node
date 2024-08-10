package usecase

import (
	lib "allora_offchain_node/lib"

	"github.com/rs/zerolog/log"
)

type UseCaseSuite struct {
	Node    lib.NodeConfig
	Metrics lib.Metrics
}

// Static method to create a new UseCaseSuite
func NewUseCaseSuite(userConfig lib.UserConfig) (*UseCaseSuite, error) {
	userConfig.ValidateConfigAdapters()
	nodeConfig, err := userConfig.GenerateNodeConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize allora client")
		return nil, err
	}
	return &UseCaseSuite{Node: *nodeConfig}, nil
}
