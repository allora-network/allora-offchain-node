package reputer_coingecko_l1_norm

import (
	"fmt"
)

type AlloraEntrypoint struct{}

func (a AlloraEntrypoint) Name() string {
	return "reputer_coingecko_l1norm"
}

func (a AlloraEntrypoint) CalcInference() {
	fmt.Println("I do nothing. from " + a.Name())
}

func (a AlloraEntrypoint) CalcForecast() {
	fmt.Println("I do nothing. from " + a.Name())
}

func (a AlloraEntrypoint) CalcLoss() {
	fmt.Println("Loss from " + a.Name())
}

func (a AlloraEntrypoint) CanInfer() bool {
	return false
}

func (a AlloraEntrypoint) CanForecast() bool {
	return false
}

func (a AlloraEntrypoint) CanCalcLoss() bool {
	return true
}

func NewAlloraEntrypoint() AlloraEntrypoint {
	return AlloraEntrypoint{
		// Initialize fields
	}
}
