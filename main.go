package main

import (
	usecase "allora_offchain_node/usecase"
)

func main() {
	spawner := usecase.NewProcessSpawner(UserConfig)
	spawner.Spawn()
}
