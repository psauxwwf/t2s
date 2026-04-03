package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"t2s/internal/config"
	"t2s/internal/dns"
	"t2s/internal/t2s"
)

const (
	_ int = iota
	defaultCode
	configCode
	initCode
	fatalCode
)

var (
	path    = flag.String("config", "", "path to config")
	timeout = flag.Int("timeout", 0, "timeout before exit")
	repair  = flag.Bool("repair", false, "repair dns error")
	save    = flag.Bool("save", false, "save default config and exit")
)

func main() {
	flag.Parse()
	if *save {
		if err := config.Default(*path); err != nil {
			fmt.Println(err)
			os.Exit(defaultCode)
		}
		return
	}

	_config, err := config.New(*path)
	if err != nil {
		fmt.Println("config parse error:", err)
		os.Exit(configCode)
	}

	fmt.Println("local relay port:", _config.RelayPort)

	_dns, err := dns.New(
		_config.Dns.Listen,
		_config.Dns.Resolvers,
		*_config.Dns.Enable,
		*_config.Dns.Render,
		*_config.Dns.Resolvectl,
		_config.Dns.Records,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(initCode)
	}
	if *repair {
		if err := _dns.Repair(); err != nil {
			fmt.Println(err)
			os.Exit(fatalCode)
		}
		return
	}

	_t2s, err := t2s.New(
		_config,
		_dns,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(initCode)
	}

	if err := _t2s.Run(
		make(chan os.Signal, 1),
		time.Duration(*timeout)*time.Second,
	); err != nil {
		fmt.Println(err)
		os.Exit(fatalCode)
	}
}
