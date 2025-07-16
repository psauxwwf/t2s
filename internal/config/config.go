package config

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"t2s/pkg/fs"
	"t2s/pkg/net"

	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
)

const (
	config = "config.yaml"
)

const (
	SocksType  = "socks"
	SshType    = "ssh"
	ChiselType = "chisel"
)

const (
	SocksProto = "socks5"
	SsProto    = "ss"
	RelayProto = "relay"
)

var ErrProtoConains = fmt.Errorf("proto must be one of %s/%s/%s",
	SocksProto, SsProto, RelayProto,
)

func protoContains(proto string) bool {
	for _, p := range []string{
		SocksProto,
		SsProto,
		RelayProto,
	} {
		if p == proto {
			return true
		}
	}
	return false
}

type proxyConfig struct {
	Type string `yaml:"type" env-description:"type of proxy <socks/ssh/chisel>" env-default:"socks"`
}

type intefaceConfig struct {
	Device       string   `yaml:"device" env-description:"device name" env-default:"tun0"`
	ExcludeNets  []string `yaml:"exclude" env-description:"not routing to proxy this nets" env-default:""`
	CustomRoutes []string `yaml:"custom_routes" env-description:"custom routes" env-default:""`
	Metric       int      `yaml:"metric" env-description:"metric priority in route" env-default:"512"`
	Sleep        int      `yaml:"sleep" env-description:"sleep before set default gateway" env-default:"0"`
}

type socksConfig struct {
	Proto    string `yaml:"proto" env-description:"proto <socks5/ss/relay>" env-default:"socks5"`
	Username string `yaml:"username" env-description:"username for socks5 proxy" env-default:""`
	Password string `yaml:"password" env-description:"password for socks5 proxy" env-default:""`
	Host     string `yaml:"host" env-description:"ip address or hostname remote proxy" env-default:"127.0.0.1"`
	Port     int    `yaml:"port" env-description:"socks5 port remote proxy" env-default:"1080"`
	Args     string `yaml:"args" env-description:"socks5://username:password@host:port/<args>" env-default:""`
}

type sshConfig struct {
	Username  string   `yaml:"username" env-description:"username for ssh" env-default:""`
	Host      string   `yaml:"host" env-description:"host for ssh" env-default:""`
	Port      int      `yaml:"port" env-description:"removte ssh port" env-default:"22"`
	Args      []string `yaml:"args" env-description:"extra args for ssh like -J user@jumphost" env-default:""`
	LocalPort int      `yaml:"-"`
}

type chiselConfig struct {
	Server   string `yaml:"server" env-description:"chisel server url <https://127.0.0.1>" env-default:""`
	Username string `yaml:"username" env-description:"username for chisel" env-default:""`
	Password string `yaml:"password" env-description:"password for chisel" env-default:""`
	Proxy    string `yaml:"proxy" env-description:"run chisel via proxy <<http/socks5h/socks>://username:password@ip:port>" env-default:""`
	IP       string `yaml:"-"`
}

type dnsConfig struct {
	Enable    *bool             `yaml:"enable" env-description:"enable dns server" env-default:"true"`
	Listen    string            `yaml:"listen" env-description:"listen local dns" env-default:"127.1.1.53"`
	Render    *bool             `yaml:"render" env-description:"render resolvconf on local dns" env-default:"true"`
	Resolvers []Resolver        `yaml:"resolvers" env-description:"dns resolvers" env-default:""`
	Records   map[string]string `yaml:"records" env-description:"custom records <1.3.3.7: 'leet.com'>" env-default:""`
}

type Resolver struct {
	IP    string         `yaml:"ip" env-description:"resolver ip" env-default:"1.1.1.1"`
	Proto string         `yaml:"proto" env-description:"resolver proto <tcp/udp>" env-default:"tcp"`
	Port  int            `yaml:"port" env-description:"resolver port" env-default:"53"`
	Rule  string         `yaml:"rule" env-description:"allow regular for domains" env-default:".*"`
	Re    *regexp.Regexp `yaml:"-"`
}

type Config struct {
	Proxy     proxyConfig    `yaml:"proxy" env-description:"proxy type"`
	Interface intefaceConfig `yaml:"interface" env-description:"interface params"`
	Socks     socksConfig    `yaml:"socks" env-description:"proxy via socks5"`
	Ssh       sshConfig      `yaml:"ssh" env-description:"proxy via ssh params"`
	Chisel    chiselConfig   `yaml:"chisel" env-description:"proxy via chisel"`
	Dns       dnsConfig      `yaml:"dns" env-description:"dns params"`
}

var _true = true

var _default = Config{
	Interface: intefaceConfig{
		Device: "tun0",
		ExcludeNets: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
		CustomRoutes: []string{},
		Metric:       512,
		Sleep:        0,
	},
	Proxy: proxyConfig{
		Type: "socks",
	},
	Socks: socksConfig{
		Proto: "socks5",
		Host:  "127.0.0.1",
		Port:  1080,
	},
	Ssh: sshConfig{
		Port: 22,
		Args: []string{},
	},
	Dns: dnsConfig{
		Listen: "127.1.1.53",
		Render: &_true,
		Resolvers: []Resolver{
			{IP: "1.1.1.1", Proto: "tcp", Port: 53, Rule: ""},
		},
		Records: map[string]string{
			"test.lan": "10.10.10.10",
		},
	},
}

func defaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, config)
	}
	if _user, err := user.Current(); err == nil {
		return filepath.Join(_user.HomeDir, ".config", "t2s", config)
	}
	return ""
}

func New(filename string) (*Config, error) {
	if filename == "" {
		filename = defaultPath()
	}
	log.Println("read config:", filename)

	port, err := net.RandomPort()
	if err != nil {
		port = 31888
	}
	_config := Config{
		Ssh: sshConfig{LocalPort: port},
		Dns: dnsConfig{
			Enable: new(bool),
			Render: new(bool),
		},
	}

	if err := cleanenv.ReadConfig(filename, &_config); err != nil {
		if os.IsNotExist(err) {
			if err := _default.Save(filename); err != nil {
				log.Println("default config error: %w", err)
			}
			return &_default, fmt.Errorf("use default path to config: %s", filename)
		}
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	for _, f := range []func() error{
		_config.checkProto,
		_config.prepareResolvers,
		_config.chiselLookup,
	} {
		if err := f(); err != nil {
			return nil, err
		}
	}

	return &_config, nil
}

func (c *Config) chiselLookup() error {
	if c.Proxy.Type == ChiselType {
		c.Chisel.IP = net.ResolveHost(c.Chisel.Server)
		c.Dns.Records[net.GetDomain(c.Chisel.Server)] = c.Chisel.IP
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
	data, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("failed to marshall config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o644); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	if err := fs.WriteFile(path, data); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	return nil
}
