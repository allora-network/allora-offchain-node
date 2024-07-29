package worker_coin_predictor_10min

import (
	"allora_offchain_node/lib"
	"fmt"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference() (string, error) {
	fmt.Println("Inference from " + a.name)
	return "", nil
}

func (a *AlloraEntrypoint) CalcForecast() ([]lib.ForecastResponse, error) {
	fmt.Println("I do nothing. from " + a.name)
	return []lib.ForecastResponse{}, nil
}

func (a *AlloraEntrypoint) SourceTruth() (lib.Truth, error) {
	fmt.Println("I do nothing. from " + a.name)
	return "", nil
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return false
}

func (a *AlloraEntrypoint) CanSourceTruth() bool {
	return false
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "worker_coin_predictor_10min_eth",
	}
}
