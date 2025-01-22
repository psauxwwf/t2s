package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tun2socksme/internal/config"
	"tun2socksme/internal/dns"
	"tun2socksme/internal/tun"
	"tun2socksme/internal/tun2socksme"
)

var (
	configpath = flag.String("config", "config.yaml", "path to config")
	resolvconf = flag.Bool("resolvconf", false, "replace /etc/resolv.conf")
)

func main() {
	flag.Parse()

	_config, err := config.New(*configpath)
	if err != nil {
		log.Println("config parse error:", err, "used default values")
	}

	_tun := tun.New(
		_config.Interface.Device,
		_config.Proxy.Username, _config.Proxy.Password, _config.Proxy.Host,
		_config.Proxy.Port,
	)

	_dns, err := dns.New(
		_config.Dns.Listen,
		_config.Dns.Resolvers,
	)
	if err != nil {
		log.Fatalln(err)
	}

	_tun2socksme, err := tun2socksme.New(
		_tun,
		_dns,
		_config.Interface.ExcludeNets,
		_config.Interface.Metric,
	)
	if err != nil {
		log.Fatalln(err)
	}

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, syscall.SIGINT, syscall.SIGTERM)

	defer _tun2socksme.Shutdown()

	if err := _tun2socksme.Run(); err != nil {
		log.Println(err)
	}

	<-sch
}
