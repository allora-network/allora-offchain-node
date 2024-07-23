package main

type WorkerConfig struct {
	TopicId             string `json:"topic_id"`
	InferenceEntrypoint string `json:"inference_entrypoint"`
	ForecastEntrypoint  string `json:"forecast_entrypoint"`
}

type ReputerConfig struct {
	TopicId           string `json:"topic_id"`
	ReputerEntrypoint string `json:"loss_entrypoint"`
}

type Config struct {
	Options struct {
		Wallet           string `json:"wallet"`
		RequestRetries   int    `json:"request_retries"`
		Node             string `json:"node"`
		LoopSeconds      int    `json:"loop_seconds"`
		MinStakeToRepute string `json:"min_stake_to_repute"`
	} `json:"options"`
	Worker  []WorkerConfig  `json:"worker"`
	Reputer []ReputerConfig `json:"reputer"`
}

func (c Config) ToEntrypointSet() map[string]bool {
	output := map[string]bool{}
	for _, w := range c.Worker {
		if w.InferenceEntrypoint != "" {
			output[w.InferenceEntrypoint] = true
		}
		if w.ForecastEntrypoint != "" {
			output[w.ForecastEntrypoint] = true
		}
	}
	for _, w := range c.Reputer {
		if w.ReputerEntrypoint != "" {
			output[w.ReputerEntrypoint] = true
		}

	}
	return output
}

type AlloraEntrypoint interface {
	Name() string
	CalcInference() //Inference
	CalcForecast()  //Forecast
	CalcLoss()      //ValueBundle // should be set of losses
	CanInfer() bool
	CanForecast() bool
	CanCalcLoss() bool
}

type Inference struct {
	Inference string
	// TODO import from allora-chain
}

type Forecast struct {
	ForecastElements []string
	// TODO import from allora-chain
}

type ValueBundle struct {
	CombinedValue string
	// TODO import from allora-chain
}
