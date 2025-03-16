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
	*Tun
}

func wrapChisel(
	config *config.Config,
	tun *Tun,
) *chisel {
	return &chisel{
		config.Chisel.Server, config.Chisel.Username, config.Chisel.Password, config.Chisel.Proxy,
		config.Chisel.IP,
		tun,
	}
}

func (c *chisel) Host() string { return c.ip }

func Chisel(_config *config.Config) (Tunnable, error) {
	return wrapChisel(
		_config,
		New(
			_config.Interface.Device,
			config.SocksProto,
			"", "", "127.0.0.1", "",
			1080,
		),
	), nil
}

func (c *chisel) Run() chan error {
	errch := c.Tun.Run()

	client, err := getClient(
		c.server,
		c.username,
		c.password,
		c.proxy,
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

func getClient(server, username, password, proxy string) (*client.Client, error) {
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
		Remotes: []string{"socks"},
	}
	_client, err := client.NewClient(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to create chisel client: %w", err)
	}
	// _client.Debug = true

	return _client, nil
}
