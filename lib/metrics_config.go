package lib

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	instance *Metrics
	once     sync.Once
)

type MetricsCounter struct {
	Name string
	Help string
}

type Metrics struct {
	Counters   []MetricsCounter
	CounterMap map[string]*prometheus.CounterVec
	mu         sync.RWMutex // Add mutex for map operations
	serverOnce sync.Once    // Add this for server initialization
}

// InitMetrics initializes the singleton instance with the given counters
func InitMetrics(counters []MetricsCounter) *Metrics {
	once.Do(func() {
		instance = &Metrics{ // nolint:exhaustruct
			Counters:   counters,
			CounterMap: make(map[string]*prometheus.CounterVec),
		}
		instance.RegisterMetricsCounters()
	})
	return instance
}

// GetMetrics returns the existing metrics instance or panics if not initialized
func GetMetrics() *Metrics {
	if instance == nil {
		log.Fatal().Msg("Metrics not initialized. Call InitMetrics first")
	}
	return instance
}

func (metrics *Metrics) RegisterMetricsCounters() {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	for _, counter := range metrics.Counters {
		counterVec := prometheus.NewCounterVec(
			prometheus.CounterOpts{ // nolint: exhaustruct
				Name: counter.Name,
				Help: counter.Help,
			},
			[]string{"address", "topic"},
		)

		prometheus.MustRegister(counterVec)
		metrics.CounterMap[counter.Name] = counterVec
	}
}

func (metrics *Metrics) StartMetricsServer(port string) {
	metrics.serverOnce.Do(func() {
		http.Handle("/metrics", promhttp.Handler())
		go func() {
			log.Info().Msgf("Starting metrics server on %s", port)
			srv := &http.Server{ // nolint: exhaustruct
				Addr:              port,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
				ReadHeaderTimeout: 10 * time.Second,
			}

			if err := srv.ListenAndServe(); err != nil {
				log.Error().Err(err).Msg("Could not start metric server")
				return
			}

			log.Info().Msg("Metrics server stopped")
		}()
	})
}

func (metrics *Metrics) IncrementMetricsCounter(counterName string, address string, topic uint64) {
	metrics.mu.RLock()
	counter := metrics.CounterMap[counterName]
	defer metrics.mu.RUnlock()

	if counter != nil {
		counter.WithLabelValues(address, strconv.FormatUint(topic, 10)).Inc()
		log.Debug().Msgf("Incremented counter %s for address %s and topic %d", counterName, address, topic)
	}
}
