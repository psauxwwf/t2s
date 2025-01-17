package tun2socksme

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"tun2socksme/internal/dns"
	"tun2socksme/internal/tun"
	shell "tun2socksme/pkg"
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
) Tun2socksme {
	return Tun2socksme{
		tun:         _tun,
		dns:         _dns,
		excludenets: _excludenets,
		metric:      _metric,
	}
}

func (t *Tun2socksme) Run() error {
	if err := t.tun.Run(); err != nil {
		return fmt.Errorf("run tun2socks error: %w", err)
	}
	if err := t.Prepare(); err != nil {
		return err
	}
	go func() {
		if err := t.dns.Listen(); err != nil {
			log.Fatalln("dns fatal error: %w", err)
		}
	}()
	return nil
}

func (t *Tun2socksme) Prepare() error {
	if err := t.setDefGate(); err != nil {
		return fmt.Errorf("gateway error: %w", err)
	}
	if err := t.disableRP(); err != nil {
		return fmt.Errorf("rp error: %w", err)
	}
	if err := t.setExcludeNets(); err != nil {
		return fmt.Errorf("route error: %w", err)
	}
	if err := t.setDefGateToTun(); err != nil {
		return fmt.Errorf("default route to proxy error: %w", err)
	}
	return nil
}

func (t *Tun2socksme) setDefGate() error {
	out, err := shell.New("ip", "ro", "sh").Run()
	if err != nil {
		return fmt.Errorf("failed to get default gateway: %w", err)
	}
	s := strings.Fields(strings.TrimSpace(out))
	if len(s) < 6 {
		return fmt.Errorf("failed to get default gateway")
	}
	t.defgate = &Gateway{
		address: s[2],
		device:  s[4],
	}

	metric, err := strconv.Atoi(s[len(s)-1])
	if err != nil {
		return fmt.Errorf("failed to get default metrice: %w", err)
	}
	if t.metric >= metric {
		log.Printf("default metric %d is more then existed metric %d set metric=%d", t.metric, metric, metric/2)
		t.metric = metric / 2
	}
	return nil
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

func (t *Tun2socksme) Shutdown() {
	if err := t.deleteRoutes(); err != nil {
		log.Println("delete routes error:", err)
	}
}
