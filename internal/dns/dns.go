package dns

import (
	"fmt"
	"log"
	"sync"

	"github.com/miekg/dns"
)

func lockf(m *sync.Mutex, f func() error) error {
	m.Lock()
	defer m.Unlock()
	return f()
}

type Dns struct {
	listen    string
	resolvers []server
	render    bool
	server    *dns.Server
	manager   *manager

	m sync.Mutex
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
			dns.HandleFailed(w, r)
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
	_resolvconfRender bool,
) (*Dns, error) {
	_manager, err := Manager(_listen)
	if err != nil {
		log.Println(err)
	}
	var (
		_dns = &Dns{
			resolvers: parseResolvers(_resolvers),
			render:    _resolvconfRender,
			listen:    _listen,
			manager:   _manager,
		}
		mux = dns.NewServeMux()
	)
	if len(_dns.resolvers) == 0 {
		return nil, fmt.Errorf("not set any resolvs")
	}

	mux.HandleFunc(".", _dns.resolv)

	_dns.server = &dns.Server{
		Addr:    fmt.Sprintf("%s:53", _dns.listen),
		Net:     "udp",
		Handler: mux,
	}
	return _dns, nil
}

func (d *Dns) Run() error {
	if d.render {
		if err := lockf(&d.m, d.manager.Set); err != nil {
			log.Printf("set dns error: %v", err)
		}
	}
	if err := d.server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start dns server: %w", err)
	}
	return nil
}

func (d *Dns) Stop() error {
	if d.render {
		if err := lockf(&d.m, d.manager.Revert); err != nil {
			log.Printf("revert dns error: %v", err)
		}
	}
	if err := d.server.Shutdown(); err != nil {
		return fmt.Errorf("failed to stop dns server: %w", err)
	}
	return nil
}
