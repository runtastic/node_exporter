package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"os"
)

type oomCollector struct {
	metric []typedDesc
}

func init() {
	registerCollector("oom", defaultDisabled, NewOomCollector)
}

func NewOomCollector() (Collector, error) {
	const subsystem = "oom"

	return &oomCollector{
		metric: []typedDesc{
			{prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "count"), "counter for out of memory occurrences.", nil, nil), prometheus.CounterValue},
		},
	}, nil
}

func (c *oomCollector) Update(ch chan<- prometheus.Metric) error {
	oom_count, err := grepFile(getSyslogFile(), []byte("Killed process"))
	if err != nil {
		return fmt.Errorf("couldn't get oom value: %s", err)
	}
	ch <- c.metric[0].mustNewConstMetric(oom_count)
	return err
}

func grepFile(file string, pat []byte) (oom_count float64, err error) {
	patCount := float64(0)
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), pat) {
			patCount++
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return patCount, nil
}

func getSyslogFile() (logfile string) {
	syslogfile := [2]string{"/var/log/kern.log", "/var/log/messages"}

	for _, file := range syslogfile {
		_, err := os.Stat(file)
		if err == nil {
			return file
		}
	}

	log.Fatal("no syslog file found")
	return ""
}
