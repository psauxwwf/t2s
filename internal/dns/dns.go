package dns

import (
	"errors"
	"fmt"
	"log/slog"
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
	enable    bool
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
			continue
			// return err
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
	_enable bool,
	render, resolvectl bool,
	_records map[string]string,
) (*Dns, error) {
	_manager, err := Manager(
		_listen,
		render,
		resolvectl,
	)
	if err != nil {
		return nil, err
	}
	var (
		_dns = &Dns{
			listen:    _listen,
			resolvers: _resolvers,
			enable:    _enable,
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
	if d.enable {
		if err := lockf(&d.m, d.manager.Set); err != nil {
			slog.Warn("set dns error", "error", err)
		}
		if err := d.server.ListenAndServe(); err != nil {
			return fmt.Errorf("failed to start dns server: %w", err)
		}
	}
	return nil
}

func (d *Dns) Stop() error {
	if d.enable {
		if err := lockf(&d.m, d.manager.Revert); err != nil {
			slog.Warn("revert dns error", "error", err)
		}
		if err := d.server.Shutdown(); err != nil {
			return fmt.Errorf("failed to stop dns server: %w", err)
		}
	}
	return nil
}

func (d *Dns) Repair() error {
	if err := lockf(&d.m, d.manager.Repair); err != nil {
		return fmt.Errorf("failed to repair dns: %w", err)
	}
	return nil
}
