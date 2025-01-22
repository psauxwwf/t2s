package tun2socksme

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"tun2socksme/internal/dns"
	"tun2socksme/internal/tun"
	"tun2socksme/pkg/shell"
)

type Gateway struct {
	device  string
	address string
}

type Tun2socksme struct {
	tun         *tun.Tun
	dns         *dns.Dns
	defgate     *Gateway
	excludenets []string
	metric      int
}

func New(
	_tun *tun.Tun,
	_dns *dns.Dns,
	_excludenets []string,
	_metric int,
) (*Tun2socksme, error) {
	s, err := getRoute()
	if err != nil {
		return nil, err
	}
	return &Tun2socksme{
		tun:         _tun,
		dns:         _dns,
		excludenets: _excludenets,
		metric:      getMertic(s, _metric),
		defgate: &Gateway{
			address: s[2],
			device:  s[4],
		},
	}, nil
}

func (t *Tun2socksme) Run() error {
	if err := t.tun.Run(); err != nil {
		return fmt.Errorf("run tun2socks error: %w", err)
	}
	if err := t.Prepare(); err != nil {
		return fmt.Errorf("prepare error: %w", err)
	}
	go func() {
		if err := t.dns.Run(); err != nil {
			log.Println("dns fatal error: %w", err)
		}
	}()
	return nil
}

func (t *Tun2socksme) Prepare() error {
	if err := t.disableRP(); err != nil {
		log.Printf("rp error: %v", err)
	}
	if err := t.setExcludeNets(); err != nil {
		return fmt.Errorf("route error: %w", err)
	}
	if err := t.setDefGateToTun(); err != nil {
		return fmt.Errorf("default route to proxy error: %w", err)
	}
	return nil
}

func (t *Tun2socksme) Shutdown() {
	var funcs = []func() error{
		t.deleteRoutes,
		t.tun.Stop,
		t.dns.Stop,
	}
	for _, f := range funcs {
		if err := f(); err != nil {
			log.Println(err)
		}
	}
}

func (t *Tun2socksme) setExcludeNets() error {
	if _, err := shell.New("ip", "ro", "add", t.tun.Host, "via", t.defgate.address, "dev", t.defgate.device).Run(); err != nil {
		return fmt.Errorf("failed to set route %s via %s", t.tun.Host, t.defgate.device)
	}
	for _, net := range t.excludenets {
		if _, err := shell.New("ip", "ro", "add", net, "via", t.defgate.address, "dev", t.defgate.device).Run(); err != nil {
			return fmt.Errorf("failed to set route %s via %s", net, t.defgate.device)
		}
	}
	return nil
}

func (t *Tun2socksme) setDefGateToTun() error {
	if _, err := shell.New("ip", "link", "set", t.tun.Device, "up").Run(); err != nil {
		return fmt.Errorf("failed to up %s device: %w", t.tun.Device, err)
	}
	if _, err := shell.New("ip", "route", "add", "default", "dev", t.tun.Device, "proto", "static", "metric", fmt.Sprint(t.metric)).Run(); err != nil {
		return fmt.Errorf("failed to set default route via %s: %w", t.tun.Device, err)
	}
	return nil
}

func (t *Tun2socksme) deleteRoutes() error {
	var err error
	if _, _err := shell.New("ip", "ro", "del", t.tun.Host).Run(); _err != nil {
		err = fmt.Errorf("failed to delete route %s: %w", t.tun.Host, _err)
	}
	for _, net := range t.excludenets {
		if _, _err := shell.New("ip", "ro", "del", net).Run(); _err != nil {
			err = fmt.Errorf("failed to delete route %s: %w", net, _err)
		}
	}
	return err
}

func (t *Tun2socksme) disableRP() error {
	if _, err := shell.New("sysctl", "net.ipv4.conf.all.rp_filter=0").Run(); err != nil {
		return fmt.Errorf("failed disable rp: %w", err)
	}
	if _, err := shell.New("sysctl", fmt.Sprintf("net.ipv4.conf.%s.rp_filter=0", t.defgate.device)).Run(); err != nil {
		return fmt.Errorf("failed disable rp: %w", err)
	}
	return nil
}

func getMertic(out []string, metric int) int {
	for i, entry := range out {
		if entry != "metric" {
			continue
		}
		if i+1 >= len(out) {
			break
		}
		if m, err := strconv.Atoi(out[i+1]); err == nil && metric >= m {
			log.Printf("default metric %d is more then existed metric %d set metric=%d", metric, m, m/2)
			return m / 2
		}
		break
	}
	return metric
}

func getRoute() ([]string, error) {
	out, err := shell.New("ip", "ro", "sh").Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get default gateway: %w", err)
	}
	splited := strings.Fields(strings.TrimSpace(out))
	if len(splited) < 6 {
		return nil, fmt.Errorf("failed to get default gateway")
	}
	return splited, nil
}
