package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"t2s/internal/config"
	"t2s/internal/dns"
	"t2s/internal/t2s"
)

const (
	_ int = iota
	defaultCode
	configCode
	initCode
	fatalCode
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newExitError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

func main() {
	if err := fang.Execute(context.Background(), rootCmd()); err != nil {
		if err, ok := errors.AsType[*exitError](err); ok {
			os.Exit(err.code)
		}
		os.Exit(fatalCode)
	}
}

func rootCmd() *cobra.Command {
	var (
		path    string
		timeout int
		level   string
	)

	runE := func(_ *cobra.Command, _ []string) error {
		return run(path, timeout, false)
	}

	root := &cobra.Command{
		Use:   "t2s",
		Short: "Route system traffic through proxy tunnel",
		Long: `t2s creates a local tun interface, starts a tun2socks relay, and routes
default traffic through a selected proxy backend.

Supported backends:
  - socks (socks5 / shadowsocks / gost relay)
  - ssh
  - chisel
  - dnstt
  - tor (via local socks endpoint)

DNS features:
  - local DNS listener
  - resolver rules (domain-based routing)
  - custom static records
  - systemd-resolved integration / recovery

Command model:
  - run    start tunnel and routing (default when no subcommand)
  - save   write default config file and exit
  - repair repair resolver state (/etc/resolv.conf + resolvectl)

Privileges:
  - run/repair require root (superuser)

Global flags:
  --config <path>   path to config yaml
  --timeout <sec>   stop after timeout seconds (0 = no timeout)

Default config path:
  ~/.config/t2s/config.yaml

Modes and specifics:
  - socks (socks5): routes traffic through a standard socks5 endpoint.
  - socks (ss): shadowsocks mode (set socks.proto=ss).
  - socks (relay): gost relay mode (set socks.proto=relay).
  - ssh: creates local socks tunnel over SSH and routes via tun interface.
  - ssh + cloudflared: use ssh args + custom_routes + startup sleep.
  - chisel: uses chisel socks backend (TLS/web-friendly transport).
  - chisel via proxy: set chisel.proxy and exclude proxy host from tun route.
  - dnstt: DNS-tunnel backend, requires resolver/pubkey/domain.
  - tor: use local tor socks port; exclude tor bridge/public IPs to avoid loops.

Operational notes:
  - run/repair require superuser privileges.
  - when running over SSH, add current SSH peer IP to interface.exclude.
  - use resolver rules + custom records for DNS leak control.
`,
		Example: `  # Generate default config
  t2s save --config /etc/t2s/config.yaml

  # Start tunnel (same as: t2s run)
  t2s --config /etc/t2s/config.yaml

  # Start and auto-exit after 10 minutes
  t2s run --config /etc/t2s/config.yaml --timeout 600

  # Repair DNS/resolver state
  t2s repair --config /etc/t2s/config.yaml

  # See detailed config variants in README.md`,
		RunE: runE,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			var parsedLevel slog.Level
			if err := parsedLevel.UnmarshalText([]byte(level)); err != nil {
				fmt.Fprintf(os.Stderr, "invalid log level %q: %v\n", level, err)
				return newExitError(2, err)
			}

			logFile, err := os.OpenFile("t2s.json", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
				return newExitError(fatalCode, err)
			}

			log := slog.New(
				slog.NewMultiHandler(
					slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
						AddSource: true,
						Level:     parsedLevel,
					}),
					slog.NewJSONHandler(logFile, &slog.HandlerOptions{
						AddSource: true,
						Level:     parsedLevel,
					}),
				),
			)
			slog.SetDefault(log)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&path, "config", "", "path to config")
	root.PersistentFlags().IntVar(&timeout, "timeout", 0, "timeout before exit")
	root.PersistentFlags().StringVar(&level, "level", "info", "log level (debug, info, warn, error)")

	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Start tunnel routing",
		Long: `Run initializes DNS manager, starts the selected proxy backend,
configures routes, then switches default route to the tun device.

This is the default action when t2s is called without a subcommand.`,
		Example: `  t2s run --config /etc/t2s/config.yaml
  t2s --config /etc/t2s/config.yaml
  t2s run --config /etc/t2s/config.yaml --timeout 300`,
		RunE: runE,
	})

	root.AddCommand(&cobra.Command{
		Use:   "repair",
		Short: "Repair DNS resolver state",
		Long: `Repair fixes resolver state used by systemd-resolved and is helpful
when /etc/resolv.conf symlink or interface DNS state is broken.`,
		Example: `  t2s repair
  t2s repair --config /etc/t2s/config.yaml`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(path, timeout, true)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "save",
		Short: "Write default config and exit",
		Long: `Save writes a full default config template.
If --config is omitted, default path is ~/.config/t2s/config.yaml.`,
		Example: `  t2s save
  t2s save --config /etc/t2s/config.yaml`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := config.Default(path); err != nil {
				return newExitError(defaultCode, err)
			}
			return nil
		},
	})

	return root
}

func run(path string, timeout int, repair bool) error {
	if os.Geteuid() != 0 {
		return newExitError(fatalCode, errors.New("this must be superuser"))
	}

	_config, err := config.New(path)
	if err != nil {
		return newExitError(configCode, fmt.Errorf("config parse error: %w", err))
	}

	_dns, err := dns.New(
		_config.Dns.Listen,
		_config.Dns.Resolvers,
		*_config.Dns.Enable,
		*_config.Dns.Render,
		*_config.Dns.Resolvectl,
		_config.Dns.Records,
	)
	if err != nil {
		return newExitError(initCode, err)
	}

	if repair {
		if err := _dns.Repair(); err != nil {
			return newExitError(fatalCode, err)
		}
		slog.Info("dns repair complete")
		return nil
	}

	slog.Info("local relay port", "port", _config.RelayPort)

	_t2s, err := t2s.New(_config, _dns)
	if err != nil {
		return newExitError(initCode, err)
	}

	if err := _t2s.Run(
		make(chan os.Signal, 1),
		time.Duration(timeout)*time.Second,
	); err != nil {
		return newExitError(fatalCode, err)
	}

	return nil
}
