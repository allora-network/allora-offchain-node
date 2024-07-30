package reputer_coingecko_l1_norm

import (
	"allora_offchain_node/lib"

	"github.com/rs/zerolog/log"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference() (string, error) {
	log.Debug().Str("name", a.name).Msg("Inference")
	return "", nil
}

func (a *AlloraEntrypoint) CalcForecast() ([]lib.ForecastResponse, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.ForecastResponse{}, nil
}

func (a *AlloraEntrypoint) SourceTruth() (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
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
