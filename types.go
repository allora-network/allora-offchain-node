package main

type TopicId = string

type ConfigOptions struct {
	Wallet           string
	RequestRetries   int
	Node             string
	LoopSeconds      int
	MinStakeToRepute string
}

type WorkerConfig struct {
	TopicId             TopicId
	InferenceEntrypoint AlloraEntrypoint
	ForecastEntrypoint  AlloraEntrypoint
}

type ReputerConfig struct {
	TopicId           TopicId
	ReputerEntrypoint AlloraEntrypoint
}

type Config struct {
	Options ConfigOptions
	Worker  []WorkerConfig
	Reputer []ReputerConfig
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
