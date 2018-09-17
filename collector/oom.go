package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"os"
	"strconv"
	"strings"
)

var pidLabelNames = []string{"time", "program"}

type oomCollector struct {
	metric []typedDesc
}

type oomDetails struct {
	Pid                 float64
	Timestamp, Program string
}

func init() {
	registerCollector("oom", defaultDisabled, NewOomCollector)
}

func NewOomCollector() (Collector, error) {
	const subsystem = "oom"

	return &oomCollector{
		metric: []typedDesc{
			{prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "count"), "counter for out of memory killer occurrences.", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "pid"), "pid that was killed by the out of memory killer.", pidLabelNames, nil), prometheus.GaugeValue},
		},
	}, nil
}

func (c *oomCollector) Update(ch chan<- prometheus.Metric) error {
	oom_count, details, err := grepFile(getSyslogFile(), []byte("Killed process"))

	if err != nil {
		return fmt.Errorf("couldn't get oom value: %s", err)
	}

	ch <- c.metric[0].mustNewConstMetric(oom_count)
	for _, detail := range details {
		ch <- c.metric[1].mustNewConstMetric(detail.Pid, detail.Timestamp, detail.Program)
	}

	return err
}

func grepFile(file string, pat []byte) (oom_count float64, details []oomDetails, err error) {
	patCount := float64(0)
	f, err := os.Open(file)
	if err != nil {
		log.Error(err)
		return patCount, nil, err
		defer f.Close()
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), pat) {
			patCount++
			logLine := scanner.Text()
			runes := []rune(logLine)
			logFields := strings.Fields(logLine)
			timestamp := string(runes[0:15])
			pid, _ := strconv.ParseFloat(logFields[8], 64)
			program := strings.Replace(strings.Replace(logFields[9], "(", "", -1), ")", "", -1)
			details = append(details, oomDetails{Pid: pid, Timestamp: timestamp, Program: program})
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return patCount, details, nil
}

func getSyslogFile() (logfile string) {
	syslogfile := [2]string{"/var/log/kern.log", "/var/log/messages"}

	for _, file := range syslogfile {
		_, err := os.Stat(file)
		if err == nil {
			return file
		}
	}

	log.Error("no syslog file found")
	return ""
}
