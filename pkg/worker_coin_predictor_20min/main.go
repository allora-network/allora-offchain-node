package coin_predictor_20min

import (
	"fmt"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference() {
	fmt.Println("Inference from " + a.name)
}

func (a *AlloraEntrypoint) CalcForecast() {
	fmt.Println("Forecast from " + a.name)
}

func (a *AlloraEntrypoint) CalcLoss() {
	fmt.Println("Loss from " + a.name)
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return true
}

func (a *AlloraEntrypoint) CanCalcLoss() bool {
	return false
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "worker_coin_predictor_20min",
	}
}
