package tun

import (
	"fmt"
	"strings"
	"sync"
	"time"

	ftmobile "github.com/samosvalishe/free-turn-proxy/mobile"

	"t2s/internal/config"
)

type freeturn struct {
	peer, obfProfile, obfKey string
	manualCaptcha            bool
	links                    []string
	ip                       string
	relayport                int
	done                     chan struct{}
	stopOnce                 sync.Once
	*Tun
}

func wrapFreeTurn(
	config *config.Config,
	tun *Tun,
) *freeturn {
	return &freeturn{
		config.FreeTurn.Peer, config.FreeTurn.ObfProfile, config.FreeTurn.ObfKey, config.FreeTurn.ManualCaptcha,
		config.FreeTurn.Links,
		config.FreeTurn.IP,
		config.RelayPort,
		nil,
		sync.Once{},
		tun,
	}
}

func (f *freeturn) Host() string { return f.ip }

func FreeTurn(_config *config.Config) (Tunnable, error) {
	return wrapFreeTurn(
		_config,
		New(
			_config.Interface.Device,
			config.ProtoSocks,
			"", "", "127.0.0.1", "",
			_config.RelayPort,
		),
	), nil
}

func (f *freeturn) Run() chan error {
	errch := f.Tun.Run()
	f.done = make(chan struct{})
	f.stopOnce = sync.Once{}

	go func() {
		if err := ftmobile.StartFlags(strings.Join(f.args(), "\n")); err != nil {
			errch <- fmt.Errorf("freeturn client error: %w", err)
			return
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-f.done:
				return
			case <-ticker.C:
				state := ftmobile.GetState()
				if state != nil && state.State == ftmobile.StateError {
					errch <- fmt.Errorf("freeturn client error: %s", state.ErrMsg)
					return
				}
			}
		}
	}()

	time.Sleep(time.Second)
	return errch
}

func (f *freeturn) Stop() error {
	if f.done != nil {
		f.stopOnce.Do(func() { close(f.done) })
	}
	ftmobile.Stop()
	return f.Tun.Stop()
}

func (f *freeturn) args() []string {
	listen := fmt.Sprintf("127.0.0.1:%d", f.relayport)
	args := []string{
		"-listen", listen,
		"-peer", f.peer,
		"-links", strings.Join(f.links, ","),
		"-mode", "tcp",
		"-obf-profile", f.obfProfile,
	}
	if f.obfKey != "" {
		args = append(args, "-obf-key", f.obfKey)
	}
	if f.manualCaptcha {
		args = append(args, "-manual-captcha")
	}
	return args
}
