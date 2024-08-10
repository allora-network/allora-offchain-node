package lib

import (
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCounter struct {
	Name string
	Help string
}

type Metrics struct {
	Counters   []MetricsCounter
	CounterMap map[string]*prometheus.CounterVec
}

func NewMetrics(counters []MetricsCounter) *Metrics {
	return &Metrics{
		Counters:   counters,
		CounterMap: make(map[string]*prometheus.CounterVec),
	}
}

func (metrics *Metrics) RegisterMetricsCounters() {

	for _, counter := range metrics.Counters {
		counterVec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: counter.Name,
				Help: counter.Help,
			},
			[]string{"address", "topic"},
		)

		prometheus.MustRegister(counterVec)
		metrics.CounterMap[counter.Name] = counterVec
	}
}

func (metrics Metrics) StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Info().Msgf("Starting metrics server on %s", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Error().Err(err).Msg("Could not start metric server")
			return
		}

		log.Info().Msg("Metrics server stopped")
	}()
}

func (metrics *Metrics) IncrementMetricsCounter(counterName string, address string, topic uint64) {
	counter := metrics.CounterMap[counterName].WithLabelValues(address, strconv.FormatUint(topic, 10))
	counter.Inc()
	log.Info().Msgf("Incremented counter %s for address %s and topic %d", counterName, address, topic)
}
