package lib

type Truth struct {
	Label string
	Value string
}

type AlloraAdapter interface {
	Name() string
	CalcInference(WorkerConfig, int64) (string, error)
	CalcLabeledInference(WorkerConfig, int64) ([]LabeledValue, error)
	CalcForecast(WorkerConfig, int64) ([]NodeValue, error)
	GroundTruth(ReputerConfig, int64) (Truth, error)
	LabeledGroundTruth(ReputerConfig, int64) ([]Truth, error)
	LossFunction(ReputerConfig, Truth, string, map[string]string) (string, error)
	LabeledLossFunction(ReputerConfig, []Truth, []string, map[string]string) (string, error)
	IsLossFunctionNeverNegative(ReputerConfig, map[string]string) (bool, error)
	CanInfer() bool
	CanForecast() bool
	CanSourceGroundTruthAndComputeLoss() bool
}

type NodeValue struct {
	Worker string `json:"worker,omitempty"`
	Value  string `json:"value,omitempty"`
}
