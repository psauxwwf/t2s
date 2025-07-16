package main

import (
	"flag"
	"log"
	"os"
	"t2s/internal/config"
	"t2s/internal/dns"
	"t2s/internal/t2s"
	"time"
)

var (
	configpath = flag.String("config", "", "path to config")
	timeout    = flag.Int("timeout", 0, "timeout before exit")
	repair     = flag.Bool("repair", false, "repair dns error")
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
		*_config.Dns.Enable,
		*_config.Dns.Render,
		_config.Dns.Records,
	)
	if err != nil {
		log.Fatalln(err)
	}
	if *repair {
		if err := _dns.Repair(); err != nil {
			log.Fatalln(err)
		}
		return
	}

	_t2s, err := t2s.New(
		_config,
		_dns,
	)
	if err != nil {
		log.Fatalln(err)
	}

	if err := _t2s.Run(
		make(chan os.Signal, 1),
		time.Duration(*timeout)*time.Second,
	); err != nil {
		log.Println(err)
	}
}
