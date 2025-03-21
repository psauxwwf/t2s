package dns

import (
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"t2s/internal/config"

	"github.com/miekg/dns"
)

var ErrResolveFailed = errors.New("failed to resolve")

func lockf(m *sync.Mutex, f func() error) error {
	m.Lock()
	defer m.Unlock()
	return f()
}

type Dns struct {
	listen    string
	resolvers []config.Resolver
	render    bool
	server    *dns.Server
	records   map[string]string
	manager   *manager

	m sync.Mutex
}

func withoutDot(s string) string {
	return strings.TrimSuffix(strings.ToLower(s), ".")
}

func (d *Dns) resolveCustom(w dns.ResponseWriter, r *dns.Msg) error {
	for _, q := range r.Question {
		domain := withoutDot(q.Name)
		if ip, found := d.records[domain]; found && q.Qtype == dns.TypeA {
			message := new(dns.Msg)
			message.SetReply(r)
			message.Answer = append(message.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(ip),
			})
			return w.WriteMsg(message)
		}
	}
	return ErrResolveFailed
}

func matchRule(re *regexp.Regexp, r *dns.Msg) error {
	for _, q := range r.Question {
		if !re.MatchString(withoutDot(q.Name)) {
			return fmt.Errorf("not mathced for rule")
		}
	}
	return nil
}

func (d *Dns) resolveExchange(w dns.ResponseWriter, r *dns.Msg) error {
	client := &dns.Client{}
	for _, resolver := range d.resolvers {

		if err := matchRule(resolver.Re, r); err != nil {
			return err
		}

		client.Net = resolver.Proto
		resp, _, err := client.Exchange(r, fmt.Sprintf("%s:%d", resolver.IP, resolver.Port))
		if err != nil {
			continue
		}
		if resp == nil || len(resp.Answer) == 0 {
			continue
		}
		return w.WriteMsg(resp)
	}
	return ErrResolveFailed
}

func (d *Dns) resolv(w dns.ResponseWriter, r *dns.Msg) {
	// for _, q := range r.Question {
	// 	domain := strings.ToLower(q.Name)
	// 	if ip, found := d.customRecords[domain]; found && q.Qtype == dns.TypeA {
	// 		message := new(dns.Msg)
	// 		message.SetReply(r)
	// 		message.Answer = append(message.Answer, &dns.A{
	// 			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
	// 			A:   net.ParseIP(ip),
	// 		})
	// 		if err := w.WriteMsg(message); err != nil {
	// 			log.Println(err)
	// 		}
	// 		return
	// 	}
	// }
	// client := &dns.Client{}
	// for _, resolver := range d.resolvers {
	// 	client.Net = resolver.proto
	// 	resp, _, err := client.Exchange(r, fmt.Sprintf("%s:%s", resolver.address, resolver.port))
	// 	if err != nil {
	// 		continue
	// 	}
	// 	if resp == nil || len(resp.Answer) == 0 {
	// 		continue
	// 	}

	// 	if err := w.WriteMsg(resp); err != nil {
	// 		log.Println(err)
	// 	}
	// 	return
	// }
	if err := d.resolveCustom(w, r); err == nil {
		return
	}
	if err := d.resolveExchange(w, r); err == nil {
		return
	}
	dns.HandleFailed(w, r)
}

func New(
	_listen string,
	_resolvers []config.Resolver,
	_resolvconfRender bool,
	_records map[string]string,
) (*Dns, error) {
	_manager, err := Manager(_listen)
	if err != nil {
		return nil, err
	}
	var (
		_dns = &Dns{
			resolvers: _resolvers,
			render:    _resolvconfRender,
			listen:    _listen,
			records:   _records,
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

func (d *Dns) Repair() error {
	if err := lockf(&d.m, d.manager.Repair); err != nil {
		return fmt.Errorf("failed to repair dns: %w", err)
	}
	return nil
}
