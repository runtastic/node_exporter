package collector

import (
	"bufio"
	"bytes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var (
	threshold       = kingpin.Flag("collector.filehandles.threshold", "Threshold for max open files in %.").Default("90").String()
	limitLabelNames = []string{"pid", "process_name", "max_open_files", "open_files", "percent"} // Label name(s) for pid
)

type filehandlesCollector struct {
	limit_reached_count *prometheus.Desc
	limit_reached       *prometheus.Desc
}

type pidOpenFiles struct {
	pid          string
	name         string
	maxOpenFiles float64
	openFiles    float64
	percent      float64
}

func init() {
	registerCollector("filehandles", defaultDisabled, NewFilehandlesCollector)
}

func NewFilehandlesCollector() (Collector, error) {
	const subsystem = "filehandles"

	limit_reached_count := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "limit_reached_count"),
		"Count of how many processes have reached "+string(*threshold)+"% of max open files. Labels will display values of last found process that reached this limit.",
		limitLabelNames, nil,
	)

	limit_reached := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subsystem, "limit_reached"),
		"Process that has reached "+string(*threshold)+"% of max open files.",
		limitLabelNames, nil,
	)

	return &filehandlesCollector{
		limit_reached_count: limit_reached_count,
		limit_reached:       limit_reached,
	}, nil
}

func (c *filehandlesCollector) Update(ch chan<- prometheus.Metric) error {
	// get values to update metrics
	limits, pids, err := getLimitReachedCount()
	if err != nil {
		log.Error(err)
	}

	var last_pid, last_name, last_maxOpenFiles, last_openFiles, last_percent string
	// update metrics for processes that have more open files than threshold% of it's max open files
	for _, p := range pids {
		last_pid = p.pid
		last_name = p.name
		last_maxOpenFiles = strconv.FormatFloat(p.maxOpenFiles, 'f', 0, 64)
		last_openFiles = strconv.FormatFloat(p.openFiles, 'f', 0, 64)
		last_percent = strconv.FormatFloat(p.percent, 'f', 2, 64)

		ch <- prometheus.MustNewConstMetric(
			c.limit_reached,
			prometheus.GaugeValue,
			p.openFiles,
			last_pid,
			last_name,
			last_maxOpenFiles,
			last_openFiles,
			last_percent,
		)
	}

	// update counter metric
	ch <- prometheus.MustNewConstMetric(c.limit_reached_count, prometheus.CounterValue, limits, last_pid, last_name, last_maxOpenFiles, last_openFiles, last_percent)

	return err
}

// get max open files of pid
func getMaxOpenFiles(pid string) (mof float64, err error) {
	f, err := os.Open("/proc/" + pid + "/limits")
	if err != nil {
		log.Error(err)
	}
	defer f.Close()

	var limit float64

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte("Max open files")) {
			if l, err := strconv.ParseFloat(strings.Fields(scanner.Text())[3], 64); err == nil {
				limit = l
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error(err)
	}

	return limit, nil
}

// process name of pid
func getProcessName(pid string) (processName string, err error) {
	f, err := os.Open("/proc/" + pid + "/status")
	if err != nil {
		log.Error(err)
	}
	defer f.Close()

	var pname string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte("Name:")) {
			pname = strings.Fields(scanner.Text())[1]
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error(err)
	}

	return pname, nil
}

func getLimitReachedCount() (procCount float64, pidErrors []pidOpenFiles, err error) {
	// get files in /proc directory
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		log.Fatal(err)
	}

	var errorCount float64
	errorPids := []pidOpenFiles{}

	for _, f := range files {
		filename := f.Name()
		// check if directory can be convertet to int because then it's a process directory
		if _, err := strconv.ParseInt(filename, 10, 64); err == nil {
			// get max open files (mof) of the current process
			mof, err := getMaxOpenFiles(string(filename))
			if err != nil {
				log.Error(err)
			}

			// get current open files (cof) of the current process
			cof, _ := ioutil.ReadDir("/proc/" + filename + "/fd")
			percentage := (float64(len(cof)) * 100) / float64(mof)
			// count up if process has more open files than threshold% of it's max open files
			if threshold, err := strconv.ParseFloat(*threshold, 64); err == nil {
				if percentage >= threshold {
					errorCount++

					// get process name of the current process
					pname, err := getProcessName(string(filename))
					if err != nil {
						log.Error(err)
					}

					// provide metric for processes that have more open files than threshold% of it's max open files
					errorPids = append(errorPids, pidOpenFiles{
						pid:          filename,
						name:         pname,
						maxOpenFiles: float64(mof),
						openFiles:    float64(len(cof)),
						percent:      percentage,
					})
				}
			}
		}
	}

	// return number of processes that have more open files than threshold% of it's max open files
	return errorCount, errorPids, err
}
