package tun2socksme

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"tun2socksme/internal/config"
	"tun2socksme/internal/dns"
	"tun2socksme/internal/tun"

	"tun2socksme/pkg/shell"
)

type Gateway struct {
	device  string
	address string
}

type Tun2socksme struct {
	tun     tun.Tunnable
	dns     *dns.Dns
	defgate *Gateway
	exclude []string
	metric  int
	routes  []string
}

func New(
	_config *config.Config,
	_dns *dns.Dns,
) (*Tun2socksme, error) {
	iprosh, err := getIprosh()
	if err != nil {
		return nil, err
	}

	_tun, err := getTun(_config)
	if err != nil {
		return nil, err
	}

	return &Tun2socksme{
		tun:     _tun,
		dns:     _dns,
		exclude: _config.Interface.ExcludeNets,
		routes:  _config.Interface.CustomRoutes,
		defgate: getDefgate(iprosh),
		metric: getMertic(
			iprosh,
			_config.Interface.Metric,
		),
	}, nil
}

func (t *Tun2socksme) Run() error {
	if err := t.Prepare(); err != nil {
		return fmt.Errorf("prepare error: %w", err)
	}

	var (
		errch = t.tun.Run()
		sigch = make(chan os.Signal, 1)
	)

	if err := t.Defgate(); err != nil {
		return fmt.Errorf("default route to proxy error: %w", err)
	}

	go func() {
		if err := t.dns.Run(); err != nil {
			errch <- fmt.Errorf("dns fatal error: %w", err)
		}
	}()

	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigch:
		return fmt.Errorf("received shutdown signal: %s", sig)
	case err := <-errch:
		return fmt.Errorf("tun2socks error: %w", err)
	}
}

func (t *Tun2socksme) Prepare() error {
	if err := t.disableRP(); err != nil {
		log.Printf("rp error: %v", err)
	}
	if err := t.setExcludeNets(); err != nil {
		return fmt.Errorf("route error: %w", err)
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

func (t *Tun2socksme) Defgate() error {
	if _, err := shell.New("ip", "link", "set", t.tun.Device(), "up").Run(); err != nil {
		return fmt.Errorf("failed to up %s device: %w", t.tun.Device(), err)
	}
	if _, err := shell.New("ip", "route", "add", "default", "dev", t.tun.Device(), "proto", "static", "metric", fmt.Sprint(t.metric)).Run(); err != nil {
		return fmt.Errorf("failed to set default route via %s: %w", t.tun.Device(), err)
	}
	return nil
}
func (t *Tun2socksme) customRoutesDel() error { return t.customRouteFunc("add") }
func (t *Tun2socksme) customRoutesAdd() error { return t.customRouteFunc("del") }
func (t *Tun2socksme) customRouteFunc(action string) error {
	for _, route := range t.routes {
		if _, err := shell.New(
			"ip",
			append(
				[]string{"ro", action},
				strings.Fields(strings.TrimSpace(route))...,
			)...,
		).Run(); err != nil {
			return fmt.Errorf("failed to %s override route %s: %w", action, t.routes, err)
		}
	}
	return nil
}

func (t *Tun2socksme) setExcludeNets() error {
	for _, net := range t.exclude {
		if _, err := shell.New("ip", "ro", "add", net, "via", t.defgate.address, "dev", t.defgate.device).Run(); err != nil {
			return fmt.Errorf("failed to set route %s via %s", net, t.defgate.device)
		}
	}
	if len(t.routes) != 0 {
		return t.customRoutesDel()
	}
	if _, err := shell.New("ip", "ro", "add", t.tun.Host(), "via", t.defgate.address, "dev", t.defgate.device).Run(); err != nil {
		return fmt.Errorf("failed to set route %s via %s", t.tun.Host(), t.defgate.device)
	}
	return nil
}

func (t *Tun2socksme) deleteRoutes() error {
	var err error
	for _, net := range t.exclude {
		if _, _err := shell.New("ip", "ro", "del", net).Run(); _err != nil {
			err = fmt.Errorf("failed to delete route %s: %w", net, _err)
		}
	}
	if len(t.routes) != 0 {
		return t.customRoutesAdd()
	}
	if _, _err := shell.New("ip", "ro", "del", t.tun.Host()).Run(); _err != nil {
		err = fmt.Errorf("failed to delete route %s: %w", t.tun.Host(), _err)
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

func getIprosh() ([]string, error) {
	out, err := shell.New("ip", "ro", "sh").Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get default gateway: %w", err)
	}
	if iprosh := strings.Fields(strings.TrimSpace(out)); len(iprosh) > 5 {
		return iprosh, nil
	}
	return nil, fmt.Errorf("failed to get default gateway")
}

func getMertic(iprosh []string, metric int) int {
	for i, entry := range iprosh {
		if entry != "metric" {
			continue
		}
		if i+1 >= len(iprosh) {
			break
		}
		if m, err := strconv.Atoi(iprosh[i+1]); err == nil && metric >= m {
			log.Printf("default metric %d is more then existed metric %d set metric=%d", metric, m, m/2)
			return m / 2
		}
		break
	}
	return metric
}

func getDefgate(iprosh []string) *Gateway {
	return &Gateway{
		address: iprosh[2],
		device:  iprosh[4],
	}
}

func getTun(_config *config.Config) (tun.Tunnable, error) {
	var m = map[string]func(*config.Config) (tun.Tunnable, error){
		config.Socks: tun.Socks,
		config.Ssh:   tun.Ssh,
	}

	if f, ok := m[_config.Proxy.Type]; ok {
		return f(_config)
	}
	return nil, fmt.Errorf("failed to parse proxy type: %v", _config.Proxy.Type)
}
