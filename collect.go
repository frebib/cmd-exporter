package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var (
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
)

type cmdExecutor struct {
	script   *Script
	result   Result
	err      error
	duration time.Duration
}

func (ce *cmdExecutor) Execute(ctx context.Context) error {
	start := time.Now()
	res, err := ce.script.Exec(ctx)

	ce.duration = time.Now().Sub(start)
	ce.err = err
	if res != nil {
		ce.result = *res
	}

	return err
}

func (ce *cmdExecutor) Gather() ([]*dto.MetricFamily, error) {
	if ce.err != nil {
		return nil, ce.err
	}
	if ce.result.Stdout == nil {
		return nil, errors.New("no output from command execution")
	}

	// Parse the command output into Metric Families ready to pass
	// to the gatherer to be sorted and sent back to the client.
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(ce.result.Stdout)
	if err != nil {
		return nil, err
	}

	// convert map[string]MetricFamily -> []MetricFamily
	ms := make([]*dto.MetricFamily, 0, len(mf))
	for _, metric := range mf {
		ms = append(ms, metric)
	}

	return ms, nil
}

func (ce *cmdExecutor) Meta(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		commandSuccess,
		prometheus.GaugeValue,
		ofBool(ce.result.ExitCode == 0 && ce.err == nil), // 1 if exitcode was 0, otherwise 0
		ce.script.Name,
	)
	ch <- prometheus.MustNewConstMetric(
		commandDuration,
		prometheus.GaugeValue,
		ce.duration.Seconds(),
		ce.script.Name,
	)
}

func ofBool(b bool) float64 {
	if b {
		return 1
	} else {
		return 0
	}
}

type cmdCollector struct {
	executors []*cmdExecutor
}

func (c cmdCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect calls each executor and collects the meta metrics for each execution
func (c cmdCollector) Collect(ch chan<- prometheus.Metric) {
	for _, executor := range c.executors {
		executor.Meta(ch)
	}
}

// CommandGatherer executes the configured commands, parses the output as
// metrics and gathers them in a prometheus.Gatherer, as can be exported with
// promhttp.HandlerFor(CommandGatherer{config}, ..)
type CommandGatherer struct {
	config *Config
}

func (c CommandGatherer) Gather() ([]*dto.MetricFamily, error) {
	var executors []*cmdExecutor
	registry := prometheus.NewRegistry()
	gatherers := prometheus.Gatherers{registry}

	var wg sync.WaitGroup
	for i := range c.config.Scripts {
		wg.Add(1)

		exec := cmdExecutor{
			script: &c.config.Scripts[i],
		}
		executors = append(executors, &exec)
		gatherers = append(gatherers, &exec)

		go func(exec *cmdExecutor) {
			defer wg.Done()
			exec.Execute(context.Background())
		}(&exec)
	}

	// All executors collect the meta metrics via this registry
	registry.MustRegister(&cmdCollector{executors: executors})

	// Wait for all command executions to terminate before gathering the results
	wg.Wait()

	return gatherers.Gather()
}
