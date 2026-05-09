package tun

import (
	"fmt"
	"time"

	"github.com/openlibrecommunity/olcrtc/mobile"

	"t2s/internal/config"
)

const olcrtcReadyTimeoutMillis = 30000

type olcrtc struct {
	carrier   string
	transport string
	roomID    string
	clientID  string
	key       string
	dns       string
	relayport int
	*Tun
}

func wrapOlcrtc(
	config *config.Config,
	tun *Tun,
) *olcrtc {
	return &olcrtc{
		carrier:   config.Olcrtc.Carrier,
		transport: config.Olcrtc.Transport,
		roomID:    config.Olcrtc.RoomID,
		clientID:  config.Olcrtc.ClientID,
		key:       config.Olcrtc.Key,
		dns:       config.Olcrtc.DNS,
		relayport: config.RelayPort,
		Tun:       tun,
	}
}

func (o *olcrtc) Host() string { return "" }

func Olcrtc(_config *config.Config) (Tunnable, error) {
	return wrapOlcrtc(
		_config,
		New(
			_config.Interface.Device,
			config.ProtoSocks,
			"", "", "127.0.0.1", "",
			_config.RelayPort,
		),
	), nil
}

func (o *olcrtc) Run() chan error {
	errch := o.Tun.Run()

	if o.dns != "" {
		mobile.SetDNS(o.dns)
	}

	if err := mobile.StartWithTransport(
		o.carrier,
		o.transport,
		o.roomID,
		o.clientID,
		o.key,
		o.relayport,
		"",
		"",
	); err != nil {
		errch <- fmt.Errorf("olcrtc start error: %w", err)
		return errch
	}

	if err := mobile.WaitReady(olcrtcReadyTimeoutMillis); err != nil {
		mobile.Stop()
		errch <- fmt.Errorf("olcrtc readiness error: %w", err)
		return errch
	}

	time.Sleep(time.Second * 3)
	return errch
}

func (o *olcrtc) Stop() error {
	mobile.Stop()
	if err := o.Tun.Stop(); err != nil {
		return err
	}
	return nil
}
