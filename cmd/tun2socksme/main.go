package main

import (
	"flag"
	"log"
	"tun2socksme/internal/config"
	"tun2socksme/internal/dns"
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

	_dns, err := dns.New(
		_config.Dns.Listen,
		_config.Dns.Resolvers,
		*_config.Dns.Render,
	)
	if err != nil {
		log.Fatalln(err)
	}

	_tun2socksme, err := tun2socksme.New(
		_config,
		_dns,
	)
	if err != nil {
		log.Fatalln(err)
	}

	defer _tun2socksme.Shutdown()

	if err := _tun2socksme.Run(); err != nil {
		log.Println(err)
	}
}
