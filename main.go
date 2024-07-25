package main

import (
	usecase "allora_offchain_node/usecase"
)

func main() {
	FullConfig := UserConfig.MapUserConfigToFullConfig()
	spawner := usecase.NewProcessSpawner(FullConfig)
	spawner.Spawn()
}
