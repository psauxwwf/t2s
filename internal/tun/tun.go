package tun

import (
	"fmt"
	"net"

	// "github.com/xjasonlyu/tun2socks/v2/dialer"
	// "github.com/xjasonlyu/tun2socks/v2/engine"
	"github.com/devil666face/tun2socks/dialer"
	"github.com/devil666face/tun2socks/engine"
)

type Tunnable interface {
	Run() chan error
	Device() string
	Host() string
	Stop() error
}

type Tun struct {
	engine *engine.Key
	host   string
	device string
}

func (t *Tun) Device() string { return t.device }
func (t *Tun) Host() string   { return t.host }

func New(
	_device string,
	proto string,
	username, password, host, args string,
	port int,
) *Tun {
	return &Tun{
		engine: &engine.Key{
			Device:   fmt.Sprintf("tun://%s", _device),
			LogLevel: "silent",
			Proxy: proxy(
				proto,
				username, password, host, args,
				port,
			),
		},
		host:   host,
		device: _device,
	}
}

func (t Tun) Run() chan error {
	var errch = make(chan error, 1)

	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = dialer.DialContext

	engine.Insert(t.engine)
	if err := engine.Start(); err != nil {
		errch <- fmt.Errorf("fatal error in interface engine: %w", err)
		return errch
	}
	return errch
}

func (t Tun) Stop() error {
	if err := engine.Stop(); err != nil {
		return fmt.Errorf("failed to stop interface engine: %w", err)
	}
	return nil
}

func proxy(
	proto string,
	username, password, host, args string,
	port int,
) string {
	var s = fmt.Sprintf("%s://", proto)
	if username != "" && password != "" {
		s += fmt.Sprintf("%s:%s@", username, password)
	}
	s += fmt.Sprintf("%s:%d", host, port)
	if args != "" {
		s += fmt.Sprintf("/%s", args)
	}
	return s
}
