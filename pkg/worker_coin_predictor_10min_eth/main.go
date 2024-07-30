package worker_coin_predictor_10min

import (
	"fmt"
	"math"
	"strconv"
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

func (a *AlloraEntrypoint) CalcForecast() ([]lib.NodeValue, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.NodeValue{}, nil
}

func (a *AlloraEntrypoint) SourceTruth() (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
	return "", nil
}

func (a *AlloraEntrypoint) LossFunction(sourceTruth string, inferenceValue string) string {
	fmt.Println("Loss function processing" + a.name)
    sourceTruthFloat, _ := strconv.ParseFloat(sourceTruth, 64)
    inferenceValueFloat, _ := strconv.ParseFloat(inferenceValue, 64)
    loss := math.Abs(sourceTruthFloat - inferenceValueFloat)

    return fmt.Sprintf("%f", loss)
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
