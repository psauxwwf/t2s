package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"

	"t2s/pkg/fs"
	"t2s/pkg/net"

	"gopkg.in/yaml.v3"
)

const (
	config = "config.yaml"
)

const (
	TypeSocks  = "socks"
	TypeSsh    = "ssh"
	TypeChisel = "chisel"
	TypeDnstt  = "dnstt"
)

const (
	ProtoSocks = "socks5"
	ProtoSs    = "ss"
	ProtoRelay = "relay"
)

var (
	ErrProtoConains = fmt.Errorf("proto must be one of %s/%s/%s",
		ProtoSocks, ProtoSs, ProtoRelay,
	)
	ErrNotExists = fmt.Errorf("config not found: %w", os.ErrNotExist)
)

func protoContains(proto string) bool {
	return slices.Contains([]string{
		ProtoSocks,
		ProtoSs,
		ProtoRelay,
	}, proto)
}

type Proxy struct {
	Type string `yaml:"type"`
}

type Interface struct {
	Device       string   `yaml:"device"`
	ExcludeNets  []string `yaml:"exclude"`
	CustomRoutes []string `yaml:"custom_routes"`
	Metric       int      `yaml:"metric"`
	Sleep        int      `yaml:"sleep"`
}

type Socks struct {
	Proto    string `yaml:"proto"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Args     string `yaml:"args"`
}

type Ssh struct {
	Username string   `yaml:"username"`
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	Args     []string `yaml:"args"`
}

type Chisel struct {
	Server   string `yaml:"server"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Proxy    string `yaml:"proxy"`
	IP       string `yaml:"-"`
}

type Dnstt struct {
	Resolver string `yaml:"resolver"`
	Pubkey   string `yaml:"pubkey"`
	Domain   string `yaml:"domain"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	IP       string `yaml:"-"`
}

type Dns struct {
	Enable     *bool             `yaml:"enable"`
	Listen     string            `yaml:"listen"`
	Render     *bool             `yaml:"render"`
	Resolvectl *bool             `yaml:"resolvectl"`
	Resolvers  []Resolver        `yaml:"resolvers"`
	Records    map[string]string `yaml:"records"`
}

type Resolver struct {
	IP    string         `yaml:"ip"`
	Proto string         `yaml:"proto"`
	Port  int            `yaml:"port"`
	Rule  string         `yaml:"rule"`
	Re    *regexp.Regexp `yaml:"-"`
}

type Config struct {
	Proxy     Proxy     `yaml:"proxy"`
	Interface Interface `yaml:"interface"`
	Socks     Socks     `yaml:"socks"`
	Ssh       Ssh       `yaml:"ssh"`
	Chisel    Chisel    `yaml:"chisel"`
	Dnstt     Dnstt     `yaml:"dnstt"`
	Dns       Dns       `yaml:"dns"`

	RelayPort int `yaml:"-"`
}

var _default = Config{
	Interface: Interface{
		Device: "tun0",
		ExcludeNets: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
		CustomRoutes: []string{
			"1.3.3.10/32 via 192.168.0.1 dev wlp3s0",
			"1.3.3.11/32 via 192.168.0.1 dev wlp3s0",
		},
		Metric: 512,
		Sleep:  0,
	},
	Proxy: Proxy{
		Type: "socks",
	},
	Socks: Socks{
		Proto:    "socks5",
		Username: "username",
		Password: "password",
		Host:     "1.3.3.7",
		Port:     1080,
		Args:     "nodelay=true",
	},
	Ssh: Ssh{
		Username: "user",
		Host:     "ssh.host.com",
		Port:     22,
		Args: []string{
			"-o",
			"ProxyCommand=cloudflared access ssh --hostname %h",
		},
	},
	Chisel: Chisel{
		Server:   "https://chisel.domain.xyz",
		Username: "username",
		Password: "password",
		Proxy:    "socks5h://proxy_username:proxy_password@1.3.3.7:1080",
	},
	Dnstt: Dnstt{
		Resolver: "udp://1.1.1.1:53",
		Pubkey:   "key",
		Domain:   "t.domain.xyz",
		Username: "username",
		Password: "password",
	},
	Dns: Dns{
		Enable:     new(true),
		Listen:     "127.1.1.53",
		Render:     new(true),
		Resolvectl: new(true),
		Resolvers: []Resolver{
			{IP: "1.1.1.1", Proto: "tcp", Port: 53, Rule: ".*"},
			{IP: "10.10.10.10", Proto: "udp", Port: 53, Rule: `.*github\.com`},
		},
		Records: map[string]string{
			"test.lan": "10.10.10.10",
		},
	},
}

func defaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, config), nil
	}
	_user, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to detect config path: %w", err)
	}
	return filepath.Join(_user.HomeDir, ".config", "t2s", config), nil
}

func Default(path string) error {
	if path == "" {
		_path, err := defaultPath()
		if err != nil {
			return err
		}
		path = _path
	}
	return _default.Save(path)
}

func New(filename string) (*Config, error) {
	var (
		_config Config
	)

	if filename == "" {
		_path, err := defaultPath()
		if err != nil {
			return nil, err
		}
		filename = _path
	}

	port, err := net.RandomPort()
	if err != nil {
		return nil, err
	}
	_config.RelayPort = port

	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotExists
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &_config); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	for _, f := range []func() error{
		_config.checkProto,
		_config.prepareResolvers,
		_config.lookup,
	} {
		if err := f(); err != nil {
			return nil, err
		}
	}

	return &_config, nil
}

func (c *Config) lookup() error {
	if c.Proxy.Type == TypeChisel {
		c.Chisel.IP = net.ResolveHost(c.Chisel.Server)
		c.Dns.Records[net.GetDomain(c.Chisel.Server)] = c.Chisel.IP
	}
	if c.Proxy.Type == TypeDnstt {
		u, err := url.Parse(c.Dnstt.Resolver)
		if err != nil {
			c.Dnstt.IP = net.ToIP(c.Dnstt.Resolver)
			return nil
		}

		switch u.Scheme {
		case "udp", "dot":
			c.Dnstt.IP = net.ToIP(u.Host)
		case "https":
			c.Dnstt.IP = net.ResolveHost(c.Dnstt.Resolver)
		default:
			c.Dnstt.IP = net.ToIP(c.Dnstt.Resolver)
		}
	}
	return nil
}

func (c *Config) checkProto() error {
	if !protoContains(c.Socks.Proto) {
		return ErrProtoConains
	}
	return nil
}

func (c *Config) prepareResolvers() (err error) {
	for i, r := range c.Dns.Resolvers {
		if r.Re, err = regexp.Compile(r.Rule); err != nil {
			return fmt.Errorf("failed to parse rule: %w", err)
		}
		c.Dns.Resolvers[i] = r
	}
	return nil
}

func (c *Config) Save(path string) error {
	var err error
	if path == "" {
		path, err = defaultPath()
		if err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("failed to marshall config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	if err := fs.WriteFile(path, data); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	return nil
}
