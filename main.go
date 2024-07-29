package main

import (
	usecase "allora_offchain_node/usecase"
)

func main() {
	spawner := usecase.NewUseCaseSuite(UserConfig)
	spawner.Spawn()
}
