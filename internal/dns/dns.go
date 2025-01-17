package dns

import (
	"fmt"
	"log"
	"regexp"

	"github.com/miekg/dns"
)

var re = regexp.MustCompile(`^([^:]+):(\d+)/(tcp|udp)$`)

type server struct {
	address string
	port    string
	proto   string
}

func parse(resolver string) (server, error) {
	matches := re.FindStringSubmatch(resolver)
	if len(matches) != 4 {
		return server{}, fmt.Errorf("failed to parse dns line: %s", resolver)
	}
	_proto := matches[3]
	if _proto != "udp" && _proto != "tcp" {
		return server{}, fmt.Errorf("fauled to parse proto it must be udp/tcp")
	}
	return server{
		address: matches[1],
		port:    matches[2],
		proto:   _proto,
	}, nil
}

func parseResolvers(resolvers []string) []server {
	var servers = []server{}
	for _, resolv := range resolvers {
		server, err := parse(resolv)
		if err != nil {
			log.Println(err)
			continue
		}
		servers = append(servers, server)
	}
	return servers
}

type Dns struct {
	resolvers []server
	server    *dns.Server
}

func (d *Dns) resolv(w dns.ResponseWriter, r *dns.Msg) {
	client := &dns.Client{}
	for _, resolver := range d.resolvers {
		client.Net = resolver.proto
		resp, _, err := client.Exchange(r, fmt.Sprintf("%s:%s", resolver.address, resolver.port))
		if err != nil {
			// log.Printf("resolver %s failed or returned no answer: %s", resolver.address, err)
			continue
		}
		if resp == nil || len(resp.Answer) == 0 {
			// log.Printf("resolver %s not found record", resolver.address)
			continue
		}

		if err := w.WriteMsg(resp); err != nil {
			// log.Printf("failed to write response: %v", err)
			dns.HandleFailed(w, r) // Handle failure properly if WriteMsg fails
			return
		}
		return
	}
	// log.Println("all resolvers failed")
	dns.HandleFailed(w, r)
}

func New(
	_listen string,
	_resolvers []string,
) (*Dns, error) {
	var (
		_dns = &Dns{
			resolvers: parseResolvers(_resolvers),
		}
		mux = dns.NewServeMux()
	)
	if len(_dns.resolvers) == 0 {
		return nil, fmt.Errorf("not set any resolvs")
	}

	mux.HandleFunc(".", _dns.resolv)

	_dns.server = &dns.Server{
		Addr:    fmt.Sprintf("%s:53", _listen),
		Net:     "udp",
		Handler: mux,
	}
	return _dns, nil
}

func (d *Dns) Listen() error {
	if err := d.server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start dns server: %w", err)
	}
	return nil
}
