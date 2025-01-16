package tun

import (
	"fmt"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

type Tun struct {
	engine *engine.Key
}

func New(
	device string,
	username, password, host string,
	port int,
) Tun {
	return Tun{
		engine: &engine.Key{
			Device:   fmt.Sprintf("tun://%s", device),
			LogLevel: "silent",
			Proxy: proxy(
				username, password, host,
				port,
			),
		},
	}
}

func (t Tun) Run() error {
	engine.Insert(t.engine)
	if err := engine.Start(); err != nil {
		return fmt.Errorf("fatal error in interface engine: %w", err)
	}
	return nil
}

func proxy(
	username, password, host string,
	port int,
) string {
	var s = "socks5://"
	if username != "" && password != "" {
		s += fmt.Sprintf("%s:%s@", username, password)
	}
	s += fmt.Sprintf("%s:%d", host, port)
	return s
}
