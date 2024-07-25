package worker_coin_predictor_20min

import (
	"allora_offchain_node/types"
	"fmt"

	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference() (emissions.Inference, error) {
	fmt.Println("Inference from " + a.name)
	return emissions.Inference{}, nil
}

func (a *AlloraEntrypoint) CalcForecast() (emissions.Forecast, error) {
	fmt.Println("Forecast from " + a.name)
	return emissions.Forecast{}, nil
}

func (a *AlloraEntrypoint) SourceTruth() (types.Truth, error) {
	fmt.Println("I do nothing, from " + a.name)
	return "", nil
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return true
}

func (a *AlloraEntrypoint) CanSourceTruth() bool {
	return false
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "worker_coin_predictor_20min",
	}
}
