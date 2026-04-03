package tun

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"t2s/internal/config"

	client "github.com/jpillora/chisel/client"
)

type chisel struct {
	server, username, password, proxy string
	ip                                string
	relayport                         int
	*Tun
}

func wrapChisel(
	config *config.Config,
	tun *Tun,
) *chisel {
	return &chisel{
		config.Chisel.Server, config.Chisel.Username, config.Chisel.Password, config.Chisel.Proxy,
		config.Chisel.IP,
		config.RelayPort,
		tun,
	}
}

func (c *chisel) Host() string { return c.ip }

func Chisel(_config *config.Config) (Tunnable, error) {
	return wrapChisel(
		_config,
		New(
			_config.Interface.Device,
			config.ProtoSocks,
			"", "", "127.0.0.1", "",
			_config.RelayPort,
		),
	), nil
}

func (c *chisel) Run() chan error {
	errch := c.Tun.Run()

	client, err := getChisel(
		c.server,
		c.username,
		c.password,
		c.proxy,
		c.relayport,
	)
	if err != nil {
		errch <- err
		return errch
	}
	if err := client.Start(context.Background()); err != nil {
		errch <- err
		return errch
	}
	go func() {
		if err := client.Wait(); err != nil {
			errch <- err
			return
		}
	}()
	time.Sleep(time.Second * 1)
	return errch
}

func getChisel(server, username, password, proxy string, relayport int) (*client.Client, error) {
	config := client.Config{
		Server:        server,
		Auth:          fmt.Sprintf("%s:%s", username, password),
		KeepAlive:     25 * time.Second,
		MaxRetryCount: -1,
		Headers:       http.Header{},
		Proxy:         proxy,
		TLS: client.TLSConfig{
			SkipVerify: true,
		},
		Remotes: []string{fmt.Sprintf("127.0.0.1:%d:socks", relayport)},
	}
	_client, err := client.NewClient(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create chisel client: %w", err)
	}
	// _client.Debug = true

	return _client, nil
}
