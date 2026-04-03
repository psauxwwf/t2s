package tun

import (
	"fmt"
	"net"
	_ "unsafe"

	"github.com/xjasonlyu/tun2socks/v2/dialer"
	"github.com/xjasonlyu/tun2socks/v2/engine"
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

//go:linkname engineStart github.com/xjasonlyu/tun2socks/v2/engine.start
func engineStart() error

//go:linkname engineStop github.com/xjasonlyu/tun2socks/v2/engine.stop
func engineStop() error

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
	if err := engineStart(); err != nil {
		errch <- fmt.Errorf("fatal error in interface engine: %w", err)
	}
	return errch
}

func (t Tun) Stop() error {
	if err := engineStop(); err != nil {
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
