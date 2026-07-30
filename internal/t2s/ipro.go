package t2s

import (
	"fmt"
	"log/slog"
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
	_defgate, err := getDefaultGateway(_s)
	if err != nil {
		return nil, err
	}
	return &Ipro{
		iprosh:       _iprosh,
		s:            _s,
		metric:       _metric,
		metricExists: _metricExists,
		defgate:      _defgate,
	}, nil
}

func getDefaultGateway(s []string) (Gateway, error) {
	var g Gateway
	for i, entry := range s {
		switch entry {
		case "via":
			if i+1 < len(s) {
				g.address = s[i+1]
			}
		case "dev":
			if i+1 < len(s) {
				g.device = s[i+1]
			}
		}
	}
	if g.address == "" {
		return g, fmt.Errorf("default gateway address not found in %q", strings.Join(s, " "))
	}
	if g.device == "" {
		return g, fmt.Errorf("default gateway device not found in %q", strings.Join(s, " "))
	}
	return g, nil
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
			slog.Warn("metric adjusted", "default_metric", metric, "existing_metric", m, "set_metric", m/2)
			return m / 2, metricExists
		}
		// return metric
		break
	}
	// return 0
	return metric, metricExists
}
