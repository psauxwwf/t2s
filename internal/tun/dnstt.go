package tun

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"t2s/internal/config"

	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"

	"www.bamsoftware.com/git/dnstt.git/dns"
	"www.bamsoftware.com/git/dnstt.git/noise"
	"www.bamsoftware.com/git/dnstt.git/turbotunnel"
)

const (
	numPadding          = 3
	idleTimeout         = 2 * time.Minute
	numPaddingForPoll   = 8
	initPollDelay       = 500 * time.Millisecond
	maxPollDelay        = 10 * time.Second
	pollDelayMultiplier = 2.0
	pollLimit           = 16
)

var base32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

const (
	defaultRetryAfter = 10 * time.Second
	dialTimeout       = 30 * time.Second
)

type dnstt struct {
	resolver, pubkey, domain, listen string
	ip                               string
	*Tun
}

func (d *dnstt) Host() string { return d.ip }

type dnsttclient struct {
	local  *net.TCPAddr
	pubkey []byte
	domain dns.Name
	remote net.Addr
	pconn  net.PacketConn
}

func wrapDnstt(
	config *config.Config,
	tun *Tun,
) *dnstt {
	return &dnstt{
		config.Dnstt.Resolver, config.Dnstt.Pubkey, config.Dnstt.Domain, net.JoinHostPort("127.0.0.1", fmt.Sprint(config.RelayPort)),
		config.Dnstt.IP,
		tun,
	}
}

func Dnstt(_config *config.Config) (Tunnable, error) {
	return wrapDnstt(
		_config,
		New(
			_config.Interface.Device,
			config.ProtoSocks,
			_config.Dnstt.Username, _config.Dnstt.Password, "127.0.0.1", "",
			_config.RelayPort,
		),
	), nil
}

func (d *dnstt) Run() chan error {
	errch := d.Tun.Run()

	client, err := getDnstt(
		d.resolver,
		d.pubkey,
		d.domain,
		d.listen,
	)
	if err != nil {
		errch <- err
		return errch
	}
	go func() {
		if err := client.run(); err != nil {
			errch <- err
			return
		}
	}()
	time.Sleep(time.Second * 1)
	return errch
}

func getDnstt(
	resolver,
	pubkey,
	domain,
	listen string,
) (*dnsttclient, error) {
	_domain, err := dns.ParseName(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to parse domain: %w", err)
	}
	_local, err := net.ResolveTCPAddr("tcp", listen)
	if err != nil {
		return nil, fmt.Errorf("failed to listen address: %w", err)
	}
	_pubkey, err := noise.DecodeKey(pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pubkey: %w", err)
	}
	if len(_pubkey) == 0 {
		return nil, fmt.Errorf("len of pubkey is 0")
	}

	_remote, _pconn, err := resolverConn(resolver)
	if err != nil {
		return nil, fmt.Errorf("failed to create dnstt obj: %w", err)
	}

	return &dnsttclient{
		pubkey: _pubkey,
		domain: _domain,
		local:  _local,
		remote: _remote,
		pconn:  NewDNSPacketConn(_pconn, _remote, _domain),
	}, nil
}

func resolverConn(resolver string) (net.Addr, net.PacketConn, error) {
	u, err := url.Parse(resolver)
	if err != nil {
		return nil, nil, fmt.Errorf("parse resolver: %w", err)
	}

	switch u.Scheme {
	case "", "udp":
		addr := resolver
		if u.Scheme == "udp" {
			addr = hostPortFromURL(u, "53")
		}
		remote, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve udp error: %w", err)
		}
		conn, err := net.ListenUDP("udp", nil)
		if err != nil {
			return nil, nil, fmt.Errorf("udp listen error: %w", err)
		}
		return remote, conn, nil
	case "dot":
		addr := hostPortFromURL(u, "853")
		conn, err := NewTLSPacketConn(addr, (&tls.Dialer{}).DialContext)
		if err != nil {
			return nil, nil, fmt.Errorf("create dot conn: %w", err)
		}
		return turbotunnel.DummyAddr{}, conn, nil
	case "https":
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		conn, err := NewHTTPPacketConn(transport, resolver, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("create doh conn: %w", err)
		}
		return turbotunnel.DummyAddr{}, conn, nil
	default:
		return nil, nil, fmt.Errorf("unsupported resolver scheme: %q", u.Scheme)
	}
}

func hostPortFromURL(u *url.URL, defaultPort string) string {
	host := u.Hostname()
	if host == "" {
		return u.Host
	}
	port := u.Port()
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port)
}

func (d *dnsttclient) run() error {
	defer d.pconn.Close()

	ln, err := net.ListenTCP("tcp", d.local)
	if err != nil {
		return fmt.Errorf("opening local listener: %v", err)
	}
	defer ln.Close()

	mtu := dnsNameCapacity(d.domain) - 8 - 1 - numPadding - 1
	if mtu < 80 {
		return fmt.Errorf("domain %s leaves only %d bytes for payload", d.domain, mtu)
	}
	slog.Info("effective MTU", "mtu", mtu)

	conn, err := kcp.NewConn2(d.remote, nil, 0, 0, d.pconn)
	if err != nil {
		return fmt.Errorf("opening KCP conn: %v", err)
	}
	defer func() {
		slog.Debug("end session", "conv", fmt.Sprintf("%08x", conn.GetConv()))
		conn.Close()
	}()
	slog.Info("begin session", "conv", fmt.Sprintf("%08x", conn.GetConv()))

	conn.SetStreamMode(true)

	conn.SetNoDelay(
		0,
		0,
		0,
		1,
	)
	conn.SetWindowSize(turbotunnel.QueueSize/2, turbotunnel.QueueSize/2)
	if rc := conn.SetMtu(mtu); !rc {
		panic(rc)
	}

	rw, err := noise.NewClient(conn, d.pubkey)
	if err != nil {
		return err
	}

	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveTimeout = idleTimeout
	smuxConfig.MaxStreamBuffer = 1 * 1024 * 1024
	sess, err := smux.Client(rw, smuxConfig)
	if err != nil {
		return fmt.Errorf("opening smux session: %v", err)
	}
	defer sess.Close()

	for {
		local, err := ln.Accept()
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				continue
			}
			return err
		}
		go func() {
			defer local.Close()
			err := handle(local.(*net.TCPConn), sess, conn.GetConv())
			if err != nil {
				slog.Warn("handle", "error", err)
			}
		}()
	}
}

func dnsNameCapacity(domain dns.Name) int {
	capacity := 255
	capacity -= 1
	for _, label := range domain {

		capacity -= len(label) + 1
	}
	capacity = capacity * 63 / 64
	capacity = capacity * 5 / 8
	return capacity
}

func handle(local *net.TCPConn, sess *smux.Session, conv uint32) error {
	stream, err := sess.OpenStream()
	if err != nil {
		return fmt.Errorf("session %08x opening stream: %v", conv, err)
	}
	defer func() {
		slog.Debug("end stream", "conv", fmt.Sprintf("%08x", conv), "stream_id", stream.ID())
		stream.Close()
	}()
	slog.Debug("begin stream", "conv", fmt.Sprintf("%08x", conv), "stream_id", stream.ID())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(stream, local)
		if err == io.EOF {

			err = nil
		}
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			slog.Warn("copy stream<-local", "conv", fmt.Sprintf("%08x", conv), "stream_id", stream.ID(), "error", err)
		}
		local.CloseRead()
		stream.Close()
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(local, stream)
		if err == io.EOF {

			err = nil
		}
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			slog.Warn("copy local<-stream", "conv", fmt.Sprintf("%08x", conv), "stream_id", stream.ID(), "error", err)
		}
		local.CloseWrite()
	}()
	wg.Wait()

	return err
}

type DNSPacketConn struct {
	clientID turbotunnel.ClientID
	domain   dns.Name
	pollChan chan struct{}
	*turbotunnel.QueuePacketConn
}

func NewDNSPacketConn(transport net.PacketConn, addr net.Addr, domain dns.Name) *DNSPacketConn {
	clientID := turbotunnel.NewClientID()
	c := &DNSPacketConn{
		clientID:        clientID,
		domain:          domain,
		pollChan:        make(chan struct{}, pollLimit),
		QueuePacketConn: turbotunnel.NewQueuePacketConn(clientID, 0),
	}
	go func() {
		err := c.recvLoop(transport)
		if err != nil {
			slog.Warn("recvLoop", "error", err)
		}
	}()
	go func() {
		err := c.sendLoop(transport, addr)
		if err != nil {
			slog.Warn("sendLoop", "error", err)
		}
	}()
	return c
}

func dnsResponsePayload(resp *dns.Message, domain dns.Name) []byte {
	if resp.Flags&0x8000 != 0x8000 {

		return nil
	}
	if resp.Flags&0x000f != dns.RcodeNoError {
		return nil
	}

	if len(resp.Answer) != 1 {
		return nil
	}
	answer := resp.Answer[0]

	_, ok := answer.Name.TrimSuffix(domain)
	if !ok {

		return nil
	}

	if answer.Type != dns.RRTypeTXT {

		return nil
	}
	payload, err := dns.DecodeRDataTXT(answer.Data)
	if err != nil {
		return nil
	}

	return payload
}

func nextPacket(r *bytes.Reader) ([]byte, error) {
	for {
		var n uint16
		err := binary.Read(r, binary.BigEndian, &n)
		if err != nil {

			return nil, err
		}
		p := make([]byte, n)
		_, err = io.ReadFull(r, p)

		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return p, err
	}
}

func (c *DNSPacketConn) recvLoop(transport net.PacketConn) error {
	for {
		var buf [4096]byte
		n, addr, err := transport.ReadFrom(buf[:])
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				slog.Debug("ReadFrom temporary error", "error", err)
				continue
			}
			return err
		}

		resp, err := dns.MessageFromWireFormat(buf[:n])
		if err != nil {
			slog.Debug("MessageFromWireFormat", "error", err)
			continue
		}

		payload := dnsResponsePayload(&resp, c.domain)

		r := bytes.NewReader(payload)
		any := false
		for {
			p, err := nextPacket(r)
			if err != nil {
				break
			}
			any = true
			c.QueuePacketConn.QueueIncoming(p, addr)
		}

		if any {
			select {
			case c.pollChan <- struct{}{}:
			default:
			}
		}
	}
}

func chunks(p []byte, n int) [][]byte {
	var result [][]byte
	for len(p) > 0 {
		sz := min(len(p), n)
		result = append(result, p[:sz])
		p = p[sz:]
	}
	return result
}

func (c *DNSPacketConn) send(transport net.PacketConn, p []byte, addr net.Addr) error {
	var decoded []byte
	{
		if len(p) >= 224 {
			return fmt.Errorf("too long")
		}
		var buf bytes.Buffer

		buf.Write(c.clientID[:])
		n := numPadding
		if len(p) == 0 {
			n = numPaddingForPoll
		}

		buf.WriteByte(byte(224 + n))
		io.CopyN(&buf, rand.Reader, int64(n))

		if len(p) > 0 {
			buf.WriteByte(byte(len(p)))
			buf.Write(p)
		}
		decoded = buf.Bytes()
	}

	encoded := make([]byte, base32Encoding.EncodedLen(len(decoded)))
	base32Encoding.Encode(encoded, decoded)
	encoded = bytes.ToLower(encoded)
	labels := chunks(encoded, 63)
	labels = append(labels, c.domain...)
	name, err := dns.NewName(labels)
	if err != nil {
		return err
	}

	var id uint16
	binary.Read(rand.Reader, binary.BigEndian, &id)
	query := &dns.Message{
		ID:    id,
		Flags: 0x0100,
		Question: []dns.Question{
			{
				Name:  name,
				Type:  dns.RRTypeTXT,
				Class: dns.ClassIN,
			},
		},

		Additional: []dns.RR{
			{
				Name:  dns.Name{},
				Type:  dns.RRTypeOPT,
				Class: 4096,
				TTL:   0,
				Data:  []byte{},
			},
		},
	}
	buf, err := query.WireFormat()
	if err != nil {
		return err
	}

	_, err = transport.WriteTo(buf, addr)
	return err
}

func (c *DNSPacketConn) sendLoop(transport net.PacketConn, addr net.Addr) error {
	pollDelay := initPollDelay
	pollTimer := time.NewTimer(pollDelay)
	for {
		var p []byte
		outgoing := c.QueuePacketConn.OutgoingQueue(addr)
		pollTimerExpired := false

		select {
		case p = <-outgoing:
		default:
			select {
			case p = <-outgoing:
			case <-c.pollChan:
			case <-pollTimer.C:
				pollTimerExpired = true
			}
		}

		if len(p) > 0 {

			select {
			case <-c.pollChan:
			default:
			}
		}

		if pollTimerExpired {

			pollDelay = min(time.Duration(float64(pollDelay)*pollDelayMultiplier), maxPollDelay)
		} else {

			if !pollTimer.Stop() {
				<-pollTimer.C
			}
			pollDelay = initPollDelay
		}
		pollTimer.Reset(pollDelay)

		err := c.send(transport, p, addr)
		if err != nil {
			slog.Debug("send", "error", err)
			continue
		}
	}
}

type HTTPPacketConn struct {
	client    *http.Client
	urlString string

	notBefore     time.Time
	notBeforeLock sync.RWMutex

	*turbotunnel.QueuePacketConn
}

func NewHTTPPacketConn(rt http.RoundTripper, urlString string, numSenders int) (*HTTPPacketConn, error) {
	c := &HTTPPacketConn{
		client: &http.Client{
			Transport: rt,
			Timeout:   1 * time.Minute,
		},
		urlString:       urlString,
		QueuePacketConn: turbotunnel.NewQueuePacketConn(turbotunnel.DummyAddr{}, 0),
	}
	for range numSenders {
		go c.sendLoop()
	}
	return c, nil
}

func (c *HTTPPacketConn) send(p []byte) error {
	req, err := http.NewRequest(http.MethodPost, c.urlString, bytes.NewReader(p))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("User-Agent", "")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if ct := resp.Header.Get("Content-Type"); ct != "application/dns-message" {
			return fmt.Errorf("unknown HTTP response Content-Type %q", ct)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64000))
		if err == nil {
			c.QueuePacketConn.QueueIncoming(body, turbotunnel.DummyAddr{})
		}
	default:
		now := time.Now()
		var retryAfter time.Time
		if value := resp.Header.Get("Retry-After"); value != "" {
			retryAfter, err = parseRetryAfter(value, now)
			if err != nil {
				slog.Warn("cannot parse Retry-After", "value", value, "error", err)
			}
		}
		if retryAfter.IsZero() {
			retryAfter = now.Add(defaultRetryAfter)
		}
		if retryAfter.Before(now) {
			slog.Warn("Retry-After in the past", "status", resp.Status, "past_by", now.Sub(retryAfter))
		} else {
			c.notBeforeLock.Lock()
			if retryAfter.After(c.notBefore) {
				slog.Warn("ceasing DoH sends", "status", resp.Status, "delay", retryAfter.Sub(now))
				c.notBefore = retryAfter
			}
			c.notBeforeLock.Unlock()
		}
	}

	return nil
}

func (c *HTTPPacketConn) sendLoop() {
	for p := range c.QueuePacketConn.OutgoingQueue(turbotunnel.DummyAddr{}) {
		c.notBeforeLock.RLock()
		notBefore := c.notBefore
		c.notBeforeLock.RUnlock()
		if wait := notBefore.Sub(time.Now()); wait > 0 {
			continue
		}

		if err := c.send(p); err != nil {
			slog.Warn("HTTP sendLoop", "error", err)
		}
	}
}

func parseRetryAfter(value string, now time.Time) (time.Time, error) {
	if t, err := http.ParseTime(value); err == nil {
		return t, nil
	}
	i, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(time.Duration(i) * time.Second), nil
}

type TLSPacketConn struct {
	*turbotunnel.QueuePacketConn
}

func NewTLSPacketConn(addr string, dialTLSContext func(ctx context.Context, network, addr string) (net.Conn, error)) (*TLSPacketConn, error) {
	dial := func() (net.Conn, error) {
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		defer cancel()
		return dialTLSContext(ctx, "tcp", addr)
	}

	conn, err := dial()
	if err != nil {
		return nil, err
	}
	c := &TLSPacketConn{QueuePacketConn: turbotunnel.NewQueuePacketConn(turbotunnel.DummyAddr{}, 0)}
	go func() {
		defer c.Close()
		for {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				if err := c.recvLoop(conn); err != nil {
					slog.Warn("DoT recvLoop", "error", err)
				}
				wg.Done()
			}()
			go func() {
				if err := c.sendLoop(conn); err != nil {
					slog.Warn("DoT sendLoop", "error", err)
				}
				wg.Done()
			}()
			wg.Wait()
			conn.Close()

			conn, err = dial()
			if err != nil {
				slog.Warn("DoT dial", "error", err)
				break
			}
		}
	}()
	return c, nil
}

func (c *TLSPacketConn) recvLoop(conn net.Conn) error {
	br := bufio.NewReader(conn)
	for {
		var length uint16
		if err := binary.Read(br, binary.BigEndian, &length); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		p := make([]byte, int(length))
		if _, err := io.ReadFull(br, p); err != nil {
			return err
		}
		c.QueuePacketConn.QueueIncoming(p, turbotunnel.DummyAddr{})
	}
}

func (c *TLSPacketConn) sendLoop(conn net.Conn) error {
	bw := bufio.NewWriter(conn)
	for p := range c.QueuePacketConn.OutgoingQueue(turbotunnel.DummyAddr{}) {
		length := uint16(len(p))
		if int(length) != len(p) {
			return fmt.Errorf("dns message too large: %d", len(p))
		}
		if err := binary.Write(bw, binary.BigEndian, &length); err != nil {
			return err
		}
		if _, err := bw.Write(p); err != nil {
			return err
		}
		if err := bw.Flush(); err != nil {
			return err
		}
	}
	return nil
}
