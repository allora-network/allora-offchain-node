package lib

type Truth = string

type AlloraEntrypoint interface {
	Name() string
	CalcInference() (string, error)
	CalcForecast() ([]NodeValue, error)
	SourceTruth() (Truth, error) // to be interpreted on a per-topic basis
	LossFunction(sourceTruth string, inferenceValue string) string
	CanInfer() bool
	CanForecast() bool
	CanSourceTruth() bool
}

type NodeValue struct {
	Worker string `json:"worker,omitempty"`
	Value  string `json:"value,omitempty"`
}
