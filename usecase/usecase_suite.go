package usecase

import (
	lib "allora_offchain_node/lib"
	"log"
)

type UseCaseSuite struct {
	Node lib.NodeConfig
}

// Static method to create a new UseCaseSuite
func NewUseCaseSuite(userConfig lib.UserConfig) UseCaseSuite {
	userConfig.ValidateConfigEntrypoints()
	nodeConfig, err := userConfig.GenerateNodeConfig()
	if err != nil {
		log.Println("Failed to initialize allora client", err)
	}
	return UseCaseSuite{Node: *nodeConfig}
}
