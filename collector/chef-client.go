package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
	"time"
)

var (
	logFile = kingpin.Flag("collector.chef-client.logfile", "Logfile to monitor chef-client runs.").Default("/var/log/chefrun").String()
)

type chefclientCollector struct {
	metric []typedDesc
}

func init() {
	registerCollector("chef-client", defaultDisabled, NewChefclientCollector)
}

func NewChefclientCollector() (Collector, error) {
	const subsystem = "chef-client"

	return &chefclientCollector{
		metric: []typedDesc{
			{prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "last_run_status"), "Status of last chef-client run (1 is ok, 0 is not ok).", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "last_run"), "Seconds since last chef-client run (modification time of "+*logFile+").", nil, nil), prometheus.GaugeValue},
		},
	}, nil
}

func (c *chefclientCollector) Update(ch chan<- prometheus.Metric) error {
	status, timediff, err := getClientStatus(*logFile)
	if err != nil {
		return fmt.Errorf("couldn't read last chef-client run.")
	}
	ch <- c.metric[0].mustNewConstMetric(status)
	ch <- c.metric[1].mustNewConstMetric(timediff)
	return err
}

func getClientStatus(logfile string) (lastRunStatus float64, secondsSinceLastRun float64, err error) {
	file, err := os.Open(logfile)
	if err != nil {
		log.Error(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lastline string
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte("chef")) {
			lastline = scanner.Text()
		}
	}

	status, err := os.Stat(logfile)
	if err != nil {
		log.Error(err)
		return 0, 0, err
	}
	lastRun := float64(time.Now().Unix()) - float64(status.ModTime().Unix())

	var lastStatus float64
	if strings.Contains(lastline, "status=success") == true {
		lastStatus = 1
	} else {
		lastStatus = 0
	}

	return lastStatus, lastRun, nil
}
