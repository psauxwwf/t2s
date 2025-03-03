package main

import (
	"flag"
	"log"
	"os"
	"t2s/internal/config"
	"t2s/internal/dns"
	"t2s/internal/t2s"
)

var (
	configpath = flag.String("config", "", "path to config")
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

	_t2s, err := t2s.New(
		_config,
		_dns,
	)
	if err != nil {
		log.Fatalln(err)
	}

	if err := _t2s.Run(make(chan os.Signal, 1)); err != nil {
		log.Println(err)
	}
}
