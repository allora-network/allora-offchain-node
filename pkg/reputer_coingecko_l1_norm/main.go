package reputer_coingecko_l1_norm

import (
	"allora_offchain_node/lib"
	"fmt"
	"math"
	"strconv"

	"github.com/rs/zerolog/log"
)

type AlloraEntrypoint struct {
	name string
}

func (a *AlloraEntrypoint) Name() string {
	return a.name
}

func (a *AlloraEntrypoint) CalcInference(node lib.WorkerConfig, blockHeight int64) (string, error) {
	log.Debug().Str("name", a.name).Msg("Inference")
	return "", nil
}

func (a *AlloraEntrypoint) CalcForecast(node lib.WorkerConfig, blockHeight int64) ([]lib.NodeValue, error) {
	log.Debug().Str("name", a.name).Msg("Forecast")
	return []lib.NodeValue{}, nil
}

func (a *AlloraEntrypoint) SourceTruth(node lib.ReputerConfig, blockHeight int64) (lib.Truth, error) {
	log.Debug().Str("name", a.name).Msg("truth")
	return "", nil
}

func (a *AlloraEntrypoint) LossFunction(sourceTruth string, inferenceValue string) string {
	fmt.Println("Loss function processing" + a.name)
	sourceTruthFloat, _ := strconv.ParseFloat(sourceTruth, 64)
	inferenceValueFloat, _ := strconv.ParseFloat(inferenceValue, 64)
	loss := math.Abs(sourceTruthFloat - inferenceValueFloat)
	str := fmt.Sprintf("%f", loss)
	log.Debug().Str("str", str).Msg("Returned loss value")
	return str
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
