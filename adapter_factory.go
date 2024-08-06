package main

import (
	api_worker_reputer "allora_offchain_node/adapter/api/worker-reputer"
	lib "allora_offchain_node/lib"
	"fmt"
)

func NewAlloraAdapter(name string) (lib.AlloraAdapter, error) {
	switch name {
	case "api-worker-reputer":
		return api_worker_reputer.NewAlloraAdapter(), nil
	// Add other cases for different adapters here
	default:
		return nil, fmt.Errorf("unknown adapter name: %s", name)
	}
}
