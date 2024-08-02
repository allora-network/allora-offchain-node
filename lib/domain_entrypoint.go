package lib

type Truth = string

type AlloraAdapter interface {
	Name() string
	CalcInference(WorkerConfig, int64) (string, error)
	CalcForecast(WorkerConfig, int64) ([]NodeValue, error)
	SourceTruth(ReputerConfig, int64) (Truth, error) // to be interpreted on a per-topic basis
	LossFunction(sourceTruth string, inferenceValue string) string
	CanInfer() bool
	CanForecast() bool
	CanSourceTruthAndComputeLoss() bool
}

type NodeValue struct {
	Worker string `json:"worker,omitempty"`
	Value  string `json:"value,omitempty"`
}
