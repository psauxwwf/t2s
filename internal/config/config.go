package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type intefaceConfig struct {
	Device      string   `yaml:"device" env-description:"device name" env-default:"tun0"`
	ExcludeNets []string `yaml:"exclude" env-description:"not routing to proxy this nets" env-default:"10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16"`
	Metric      int      `yaml:"metric" env-description:"metric priority in route" env-default:"512"`
}

type proxyConfig struct {
	Username string `yaml:"username" env-description:"username for socks5 proxy" env-default:""`
	Password string `yaml:"password" env-description:"password for socks5 proxy" env-default:""`
	Host     string `yaml:"host" env-description:"ip address or hostname remote proxy" env-default:"127.0.0.1"`
	Port     int    `yaml:"port" env-description:"socks5 port remote proxy" env-default:"1080"`
}

type dnsConfig struct {
	Listen    string   `yaml:"listen" env-description:"listen local dns" env-default:"127.1.1.53"`
	Render    *bool    `yaml:"render" env-description:"render resolvconf on local dns" env-default:"true"`
	Resolvers []string `yaml:"resolvers" env-description:"dns resolvers" env-default:"1.1.1.1:53/tcp"`
}

type Config struct {
	Interface intefaceConfig `yaml:"interface" env-description:"interface params"`
	Proxy     proxyConfig    `yaml:"proxy" env-description:"remote proxy params"`
	Dns       dnsConfig      `yaml:"dns" env-description:"dns params"`
}

// func (c Config) String() string {
// 	return fmt.Sprintf(`device: %s
// username: %s
// host: %s
// port: %d
// exclude: %s
// metric: %d
// `,
// 		c.Device,
// 		c.Username,
// 		c.Host,
// 		c.Port,
// 		strings.Join(c.ExcludeNets, " "),
// 		c.Metric,
// 	)
// }

func New(filename string) (*Config, error) {
	var _config = Config{
		Interface: intefaceConfig{
			Device: "tun0",
			ExcludeNets: []string{
				"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
			},
			Metric: 512,
		},
		Proxy: proxyConfig{
			Host: "127.0.0.1",
			Port: 1080,
		},
		Dns: dnsConfig{
			Listen: "127.1.1.53",
			Resolvers: []string{
				"1.1.1.1:53/tcp",
			},
		},
	}
	if err := cleanenv.ReadConfig(filename, &_config); err != nil {
		if !os.IsNotExist(err) {
			return &_config, fmt.Errorf("failed to load config: %w", err)
		}
	}
	return &_config, nil
}
