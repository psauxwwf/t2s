package tun

import (
	"fmt"
	"strings"
	"tun2socksme/internal/config"
	"tun2socksme/pkg/shell"
)

type ssh struct {
	username, host, args string
	port, localPort      int
	sshPID               int
	*Tun
}

func wrapSsh(
	config *config.Config,
	tun *Tun,
) *ssh {
	return &ssh{
		config.Ssh.Username, config.Ssh.Host, config.Ssh.Args,
		config.Ssh.Port, config.Ssh.LocalPort,
		0,
		tun,
	}
}

func (s *ssh) Host() string {
	return s.host
}

func Ssh(_config *config.Config) (Tunnable, error) {
	return wrapSsh(
		_config,
		New(
			_config.Interface.Device,
			config.SocksProto,
			"", "", "127.0.0.1", "",
			_config.Ssh.LocalPort,
		),
	), nil
}

func (s *ssh) Run() chan error {
	var errch = s.Tun.Run()
	go func() {
		if _, err := shell.New("ssh", s.sshOpts()...).Run(); err != nil {
			errch <- fmt.Errorf("ssh error: %w", err)
			return
		}
	}()
	return errch
}

func (s *ssh) sshOpts() []string {
	var opts = []string{
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=5",
	}
	if s.port != 22 {
		opts = append(opts, []string{"-p", fmt.Sprint(s.port)}...)
	}
	opts = append(opts, []string{
		"-D", fmt.Sprint(s.localPort),
		fmt.Sprintf("%s@%s", s.username, s.host),
		"-N",
	}...)
	if s.args != "" {
		opts = append(opts,
			strings.Fields(strings.TrimSpace(s.args))...,
		)
	}
	return opts
}
