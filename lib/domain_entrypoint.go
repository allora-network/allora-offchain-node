package lib

type Truth = string

type AlloraEntrypoint interface {
	Name() string
	CalcInference() (string, error)
	CalcForecast() ([]ForecastResponse, error)
	SourceTruth() (Truth, error) // to be interpreted on a per-topic basis
	CanInfer() bool
	CanForecast() bool
	CanSourceTruth() bool
}

type ForecastResponse struct {
	Worker string `json:"worker,omitempty"`
	Value  string `json:"value,omitempty"`
}
