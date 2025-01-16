package main

import (
	"flag"
	"fmt"
	"log"
	"tun2socksme/internal/config"
	"tun2socksme/internal/tun"
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
	if err := _tun.Run(); err != nil {
		log.Fatalln(err)
	}
	select {}
}
