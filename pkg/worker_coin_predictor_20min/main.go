package coin_predictor_20min

import (
	"fmt"
)

type AlloraEntrypoint struct{}

var name = "worker_coin_predictor_20min"

func (a AlloraEntrypoint) Name() string {
	return name
}

func (a AlloraEntrypoint) CalcInference() {
	fmt.Println("Inference from " + a.Name())
}

func (a AlloraEntrypoint) CalcForecast() {
	fmt.Println("Forecast from " + a.Name())
}

func (a AlloraEntrypoint) CalcLoss() {
	fmt.Println("Loss from " + a.Name())
}

func (a AlloraEntrypoint) CanInfer() bool {
	return true
}

func (a AlloraEntrypoint) CanForecast() bool {
	return true
}

func (a AlloraEntrypoint) CanCalcLoss() bool {
	return false
}

func NewAlloraEntrypoint() AlloraEntrypoint {
	return AlloraEntrypoint{
		// Initialize fields
	}
}
