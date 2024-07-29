package main

import (
	usecase "allora_offchain_node/usecase"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// UNIX Time is faster and smaller than most timestamps
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Print("Starting allora offchain node...")

	spawner := usecase.NewUseCaseSuite(UserConfig)
	spawner.Spawn()
}
