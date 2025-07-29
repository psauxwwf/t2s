package t2s

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"t2s/internal/config"
	"t2s/internal/dns"
	"t2s/internal/tun"

	"t2s/pkg/shell"
)

func lockf(m *sync.Mutex, f func() error) error {
	m.Lock()
	defer m.Unlock()
	return f()
}

type Gateway struct {
	device  string
	address string
}

type Tun2socksme struct {
	ipro    *Ipro
	tun     tun.Tunnable
	dns     *dns.Dns
	exclude []string
	routes  []string
	sleep   int

	m sync.Mutex
}

func New(
	_config *config.Config,
	_dns *dns.Dns,
) (*Tun2socksme, error) {
	_ipro, err := getIpro(_config.Interface.Metric)
	if err != nil {
		return nil, err
	}

	_tun, err := getTun(_config)
	if err != nil {
		return nil, err
	}

	return &Tun2socksme{
		ipro:    _ipro,
		tun:     _tun,
		dns:     _dns,
		exclude: _config.Interface.ExcludeNets,
		routes:  _config.Interface.CustomRoutes,
		sleep:   _config.Interface.Sleep,
	}, nil
}

func (t *Tun2socksme) Run(sigch chan os.Signal, timeout time.Duration) error {
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		if err := t.Shutdown(); err != nil {
			log.Println(err)
		}
	}()

	if err := lockf(&t.m, t.Prepare); err != nil {
		return fmt.Errorf("prepare error: %w", err)
	}

	errch := t.tun.Run()

	go func() {
		if err := t.dns.Run(); err != nil {
			errch <- fmt.Errorf("dns fatal error: %w", err)
		}
	}()
	go func() {
		time.Sleep(time.Second * time.Duration(t.sleep))
		if err := t.Defgate(); err != nil {
			errch <- fmt.Errorf("default route to proxy error: %w", err)
		}
	}()
	go func() {
		<-time.After(timeout)
		if timeout != 0 {
			errch <- fmt.Errorf("timeouted")
		}
	}()

	select {
	case err := <-errch:
		return fmt.Errorf("fatal error: %s", err)
	case sig := <-sigch:
		return fmt.Errorf("recieve: %v", sig)
	}
}

func (t *Tun2socksme) Prepare() error {
	if err := t.disableRP(); err != nil {
		log.Printf("rp error: %v", err)
	}
	if err := t.addRoutes(); err != nil {
		return fmt.Errorf("route error: %w", err)
	}
	return nil
}

func (t *Tun2socksme) Shutdown() (err error) {
	var funcs = []func() error{
		t.dns.Stop,
		t.tun.Stop,
		t.deleteRoutes,
	}
	for _, f := range funcs {
		if _err := lockf(&t.m, f); _err != nil {
			if err != nil {
				err = fmt.Errorf("%w: %w", err, _err)
				continue
			}
			err = _err
		}
	}
	return
}

func (t *Tun2socksme) Defgate() error {
	if _, err := shell.New("ip", "link", "set", t.tun.Device(), "up").Run(); err != nil {
		return fmt.Errorf("failed to up %s device: %w", t.tun.Device(), err)
	}
	if !t.ipro.metricExists {
		var args = append([]string{"route", "replace"},
			t.ipro.s...)
		args = append(args, "metric", fmt.Sprint(t.ipro.metric*2))

		if _, err := shell.New("ip", args...).Run(); err != nil {
			return fmt.Errorf("failed to replace def route without metric to route with metric: %w", err)
		}
		if _, err := shell.New("ip", append([]string{"ro", "del"}, t.ipro.s...)...).Run(); err != nil {
			return fmt.Errorf("failed to delete previous route without metric: %w", err)
		}
	}
	if _, err := shell.New("ip", "route", "add", "default", "dev", t.tun.Device(), "proto", "static", "metric", fmt.Sprint(t.ipro.metric)).Run(); err != nil {
		return fmt.Errorf("failed to set default route via %s: %w", t.tun.Device(), err)
	}
	return nil
}
func (t *Tun2socksme) customRoutesDel() error { return t.customRouteFunc("del") }
func (t *Tun2socksme) customRoutesAdd() error { return t.customRouteFunc("add") }
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

func (t *Tun2socksme) addRoutes() error {
	for _, net := range t.exclude {
		if _, err := shell.New("ip", "ro", "add", net, "via", t.ipro.defgate.address, "dev", t.ipro.defgate.device).Run(); err != nil {
			return fmt.Errorf("failed to set route %s via %s", net, t.ipro.defgate.device)
		}
	}
	if len(t.routes) != 0 {
		if err := t.customRoutesAdd(); err != nil {
			return fmt.Errorf("failed to set custom routes: %w", err)
		}
		return nil
	}
	if _, err := shell.New("ip", "ro", "add", t.tun.Host(), "via", t.ipro.defgate.address, "dev", t.ipro.defgate.device).Run(); err != nil {
		return fmt.Errorf("failed to set route %s via %s", t.tun.Host(), t.ipro.defgate.device)
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
		if err := t.customRoutesDel(); err != nil {
			return fmt.Errorf("failed to delete custom routes: %w", err)
		}
		return nil
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
	if _, err := shell.New("sysctl", fmt.Sprintf("net.ipv4.conf.%s.rp_filter=0", t.ipro.defgate.device)).Run(); err != nil {
		return fmt.Errorf("failed disable rp: %w", err)
	}
	return nil
}

func getTun(_config *config.Config) (tun.Tunnable, error) {
	var m = map[string]func(*config.Config) (tun.Tunnable, error){
		config.SocksType:  tun.Socks,
		config.SshType:    tun.Ssh,
		config.ChiselType: tun.Chisel,
		config.DnsttType:  tun.Dnstt,
	}

	if f, ok := m[_config.Proxy.Type]; ok {
		return f(_config)
	}
	return nil, fmt.Errorf("failed to parse proxy type: %v", _config.Proxy.Type)
}
