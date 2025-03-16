package tun

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"t2s/internal/config"

	client "github.com/jpillora/chisel/client"
)

type chisel struct {
	server, username, password, proxy string
	*Tun
}

func wrapChisel(
	config *config.Config,
	tun *Tun,
) *chisel {
	return &chisel{
		config.Chisel.Server, config.Chisel.Username, config.Chisel.Password, config.Chisel.Proxy,
		tun,
	}
}

func (c *chisel) Host() string {
	_url, err := url.Parse(c.server)
	if err != nil {
		return ""
	}

	hostname := _url.Hostname()
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	ip, err := net.LookupIP(hostname)
	if err != nil {
		return hostname
	}

	return ip[0].String()
}

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
	var errch = c.Tun.Run()
	go func() {
		client, err := getClient(
			c.server,
			c.username,
			c.password,
			c.proxy,
		)
		if err != nil {
			errch <- err
			return
		}
		if err := client.Start(context.Background()); err != nil {
			errch <- err
			return
		}
	}()
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
	_client.Debug = true

	return _client, nil
}
