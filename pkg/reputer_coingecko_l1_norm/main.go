package reputer_coingecko_l1_norm

import (
	"allora_offchain_node/lib"
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
	fmt.Println("I do nothing. from " + a.name)
	return emissions.Inference{}, nil
}

func (a *AlloraEntrypoint) CalcForecast() (emissions.Forecast, error) {
	fmt.Println("I do nothing. from " + a.name)
	return emissions.Forecast{}, nil
}

func (a *AlloraEntrypoint) SourceTruth() (lib.Truth, error) {
	fmt.Println("Truth from " + a.name)
	return "", nil
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return false
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return false
}

func (a *AlloraEntrypoint) CanSourceTruth() bool {
	return true
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "reputer_coingecko_l1norm",
	}
}
