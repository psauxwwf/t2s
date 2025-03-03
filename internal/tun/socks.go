package tun

import (
	"t2s/internal/config"
)

type socks struct {
	*Tun
}

func wrapSocks(tun *Tun) *socks {
	return &socks{
		tun,
	}
}

func Socks(_config *config.Config) (Tunnable, error) {
	return wrapSocks(
		New(
			_config.Interface.Device,
			_config.Socks.Proto,
			_config.Socks.Username, _config.Socks.Password, _config.Socks.Host, _config.Socks.Args,
			_config.Socks.Port,
		),
	), nil
}
