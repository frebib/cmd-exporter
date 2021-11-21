package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var (
	// Namespace is the metric namespace
	Namespace = "command"

	commandSuccess = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "", "success"),
		"denotes whether the command ran successfully and exited success",
		[]string{"command"}, nil,
	)
	commandDuration = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "duration", "seconds"),
		"duration in seconds that the command execution took",
		[]string{"command"}, nil,
	)
	commandMetricCount = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "metric", "count"),
		"number of metrics returned by the command",
		[]string{"command"}, nil,
	)
)

func (s *Script) Gather() ([]*dto.MetricFamily, error) {
	var metrics []*dto.MetricFamily
	var parsed map[string]*dto.MetricFamily
	var duration time.Duration
	var count int

	labels := &dto.LabelPair{
		Name:  proto.String("command"),
		Value: proto.String(s.Name),
	}

	exitCode := -1
	start := time.Now()
	wait, stdout, err := s.Run(context.Background())
	if err != nil {
		log.Printf("failed to start command %s: %s", s.Name, err)
		goto result
	}

	// Parse the command output into Metric Families ready to pass
	// to the gatherer to be sorted and sent back to the client.
	parsed, err = new(expfmt.TextParser).TextToMetricFamilies(stdout)
	if err != nil {
		log.Printf("failed parse metrics from command %s: %s", s.Name, err)
		duration = time.Now().Sub(start)
		goto result
	}

	exitCode, err = wait()
	duration = time.Now().Sub(start)

	metrics = make([]*dto.MetricFamily, 0, len(parsed))
	for _, mf := range parsed {
		// Inject additional labels
		for _, metric := range mf.Metric {
			metric.Label = append(metric.Label, labels)
		}
		count += len(mf.Metric)
		metrics = append(metrics, mf)
	}

result:
	r := prometheus.NewPedanticRegistry()
	r.MustRegister(MetricCollector{
		prometheus.MustNewConstMetric(
			commandSuccess,
			prometheus.GaugeValue,
			ofBool(exitCode == 0 && err == nil), // 1 if exitcode was 0, otherwise 0
			s.Name,
		),
		prometheus.MustNewConstMetric(
			commandDuration,
			prometheus.GaugeValue,
			duration.Seconds(),
			s.Name,
		),
		prometheus.MustNewConstMetric(
			commandMetricCount,
			prometheus.GaugeValue,
			float64(count),
			s.Name,
		),
	})
	meta, err := r.Gather()
	metrics = append(metrics, meta...)

	return metrics, err
}

type MetricCollector []prometheus.Metric

func (mc MetricCollector) Describe(descs chan<- *prometheus.Desc) {
	for _, metric := range mc {
		descs <- metric.Desc()
	}
}

func (mc MetricCollector) Collect(metrics chan<- prometheus.Metric) {
	for _, metric := range mc {
		metrics <- metric
	}
}

func ofBool(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type gatherResult struct {
	metrics []*dto.MetricFamily
	err     error
}

func (gr gatherResult) Gather() ([]*dto.MetricFamily, error) {
	return gr.metrics, gr.err
}

// CommandGatherer executes the configured commands, parses the output as
// metrics and gathers them in a prometheus.Gatherer, as can be exported with
// promhttp.HandlerFor(CommandGatherer{config}, ..)
type CommandGatherer struct {
	config *Config
}

// Gather calls the Collect method of the registered Collectors
func (c CommandGatherer) Gather() ([]*dto.MetricFamily, error) {
	var wg sync.WaitGroup

	gatherers := make(prometheus.Gatherers, len(c.config.Scripts))

	// Run all gatherers in goroutines and collect the results into a slice.
	// This starts them all at once instead of in serial, which is all the
	// prometheus.Gatherers implementation can muster.
	wg.Add(len(c.config.Scripts))
	for i := range c.config.Scripts {
		go func(i int) {
			metrics, err := c.config.Scripts[i].Gather()
			gatherers[i] = gatherResult{metrics, err}
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Collate the multiple gatherers output in one go after they've all
	// been executed
	return gatherers.Gather()
}
