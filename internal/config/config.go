package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Device      string   `yaml:"device" env-description:"device name" env-default:"tun0"`
	Username    string   `yaml:"username" env-description:"username for socks5 proxy" env-default:""`
	Password    string   `yaml:"password" env-description:"password for socks5 proxy" env-default:""`
	Host        string   `yaml:"host" env-description:"ip address or hostname remote proxy" env-default:"127.0.0.1"`
	Port        int      `yaml:"port" env-description:"socks5 port remote proxy" env-default:"1080"`
	ExcludeNets []string `yaml:"exclude" env-description:"not routing to proxy this nets" env-default:"10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16"`
	Metric      int      `yaml:"metric" env-description:"metric priority in route" env-default:"512"`
}

func (c Config) String() string {
	return fmt.Sprintf(`device: %s
username: %s
host: %s
port: %d
exclude: %s
metric: %d
`,
		c.Device,
		c.Username,
		c.Host,
		c.Port,
		strings.Join(c.ExcludeNets, " "),
		c.Metric,
	)
}

func New(filename string) (*Config, error) {
	var _config = Config{
		Device: "tun0",
		Host:   "127.0.0.1",
		Port:   1080,
		ExcludeNets: []string{
			"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		},
		Metric: 512,
	}
	if err := cleanenv.ReadConfig(filename, &_config); err != nil {
		if !os.IsNotExist(err) {
			return &_config, fmt.Errorf("failed to load config: %w", err)
		}
	}
	return &_config, nil
}
