package config

import (
	"fmt"
	"log"
	"os"
	"tun2socksme/pkg/fs"
	"tun2socksme/pkg/net"

	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
)

const (
	Socks = "socks"
	Ssh   = "ssh"
)

type proxyConfig struct {
	Type string `yaml:"type" env-description:"type of proxy socks/ssh" env-default:"socks"`
}

type intefaceConfig struct {
	Device       string   `yaml:"device" env-description:"device name" env-default:"tun0"`
	ExcludeNets  []string `yaml:"exclude" env-description:"not routing to proxy this nets" env-default:""`
	CustomRoutes []string `yaml:"custom_routes" env-description:"custom routes" env-default:""`
	Metric       int      `yaml:"metric" env-description:"metric priority in route" env-default:"512"`
}

type socksConfig struct {
	Username string `yaml:"username" env-description:"username for socks5 proxy" env-default:""`
	Password string `yaml:"password" env-description:"password for socks5 proxy" env-default:""`
	Host     string `yaml:"host" env-description:"ip address or hostname remote proxy" env-default:"127.0.0.1"`
	Port     int    `yaml:"port" env-description:"socks5 port remote proxy" env-default:"1080"`
}

type sshConfig struct {
	Username  string `yaml:"username" env-description:"username for ssh" env-default:""`
	Host      string `yaml:"host" env-description:"host for ssh" env-default:""`
	Port      int    `yaml:"port" env-description:"removte ssh port" env-default:"22"`
	LocalPort int    `yaml:"-"`
}

type dnsConfig struct {
	Listen    string   `yaml:"listen" env-description:"listen local dns" env-default:"127.1.1.53"`
	Render    *bool    `yaml:"render" env-description:"render resolvconf on local dns" env-default:"true"`
	Resolvers []string `yaml:"resolvers" env-description:"dns resolvers" env-default:"1.1.1.1:53/tcp"`
}

type Config struct {
	Proxy     proxyConfig    `yaml:"proxy" env-description:"proxy type"`
	Interface intefaceConfig `yaml:"interface" env-description:"interface params"`
	Socks     socksConfig    `yaml:"socks" env-description:"proxy via socks5"`
	Ssh       sshConfig      `yaml:"ssh" env-description:"proxy via ssh params"`
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
		Metric: 512,
	},
	Proxy: proxyConfig{
		Type: "socks",
	},
	Socks: socksConfig{
		Host: "127.0.0.1",
		Port: 1080,
	},
	Ssh: sshConfig{
		Port: 22,
	},
	Dns: dnsConfig{
		Listen: "127.1.1.53",
		Resolvers: []string{
			"1.1.1.1:53/tcp",
		},
		Render: &_true,
	},
}

func New(filename string) (*Config, error) {
	port, err := net.RandomPort()
	if err != nil {
		log.Println("failed to get port for local ssh tunnel")
		port = 31888
	}
	_config := Config{Ssh: sshConfig{LocalPort: port}}

	if err := cleanenv.ReadConfig(filename, &_config); err != nil {
		if os.IsNotExist(err) {
			if err := _default.Save(filename); err != nil {
				log.Println("default config error: %w", err)
			}
			return &_default, fmt.Errorf("config not exists: %w", err)
		}
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &_config, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("failed to marshall config: %w", err)
	}
	if err := fs.WriteFile(path, data); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	return nil
}
