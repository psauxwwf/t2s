package main

import (
	"flag"
	"fmt"
	"log"
	"tun2socksme/internal/config"
	"tun2socksme/internal/tun"
	"tun2socksme/internal/tun2socksme"
)

var (
	configpath = flag.String("config", "config.yaml", "path to config")
)

func main() {
	flag.Parse()

	_config, err := config.New(*configpath)
	if err != nil {
		log.Println("config parse error:", err, "used default values")
	}
	fmt.Println(_config)

	_tun := tun.New(
		_config.Device,
		_config.Username, _config.Password, _config.Host,
		_config.Port,
	)

	_tun2socksme := tun2socksme.New(
		_tun,
		_config.ExcludeNets,
		_config.Metric,
	)
	if err := _tun2socksme.Run(); err != nil {
		log.Fatalln(err)
	}
	select {}
}
