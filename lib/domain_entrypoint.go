package lib

import (
	emissions "github.com/allora-network/allora-chain/x/emissions/types"
)

type Truth = string

type AlloraEntrypoint interface {
	Name() string
	CalcInference() (emissions.Inference, error)
	CalcForecast() (emissions.Forecast, error)
	SourceTruth() (Truth, error) // to be interpreted on a per-topic basis
	CanInfer() bool
	CanForecast() bool
	CanSourceTruth() bool
}
