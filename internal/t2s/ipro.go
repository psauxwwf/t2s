package t2s

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"t2s/pkg/shell"
)

func iprosh() (string, error) {
	out, err := shell.New("ip", "ro", "sh", "default").Run()
	if err != nil {
		return "", fmt.Errorf("failed to get default gateway: %w", err)
	}
	if lines := strings.Split(out, "\n"); len(lines) > 0 {
		return lines[0], nil
	}
	return "", fmt.Errorf("failed to get first line of default gateways")
}

type Ipro struct {
	iprosh       string
	s            []string
	metric       int
	metricExists bool
	defgate      Gateway
}

func getIpro(metricDefault int) (*Ipro, error) {
	_iprosh, err := iprosh()
	if err != nil {
		return nil, err
	}
	_s := strings.Fields(strings.TrimSpace(_iprosh))
	if len(_s) < 4 {
		return nil, fmt.Errorf("default gateway line is too short")
	}
	_metric, _metricExists := getMetric(_s, metricDefault)
	return &Ipro{
		iprosh:       _iprosh,
		s:            _s,
		metric:       _metric,
		metricExists: _metricExists,
		defgate: Gateway{
			address: _s[2],
			device:  _s[4],
		},
	}, nil
}

func getMetric(s []string, metric int) (int, bool) {
	metricExists := false
	for i, entry := range s {
		if entry != "metric" {
			continue
		}
		if i+1 >= len(s) {
			break
		}
		metricExists = true
		if m, err := strconv.Atoi(s[i+1]); err == nil && metric >= m {
			log.Printf("default metric %d is more then existed metric %d set metric=%d", metric, m, m/2)
			return m / 2, metricExists
		}
		// return metric
		break
	}
	// return 0
	return metric, metricExists
}
