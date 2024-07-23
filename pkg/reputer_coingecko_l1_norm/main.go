package reputer_coingecko_l1_norm

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
	fmt.Println("I do nothing. from " + a.name)
}

func (a *AlloraEntrypoint) CalcForecast() {
	fmt.Println("I do nothing. from " + a.name)
}

func (a *AlloraEntrypoint) CalcLoss() {
	fmt.Println("Loss from " + a.name)
}

func (a *AlloraEntrypoint) CanInfer() bool {
	return false
}

func (a *AlloraEntrypoint) CanForecast() bool {
	return false
}

func (a *AlloraEntrypoint) CanCalcLoss() bool {
	return true
}

func NewAlloraEntrypoint() *AlloraEntrypoint {
	return &AlloraEntrypoint{
		name: "reputer_coingecko_l1norm",
	}
}
