package main

import (
	"log"

	"github.com/miekg/dns"
)

func forwardDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	client := new(dns.Client)
	client.Net = "tcp"

	// Define upstream servers
	upstreamPrimary := "1.1.1.1:53"
	upstreamSecondary := "192.168.200.1:53"

	// First, try to forward the query to 1.1.1.1 via TCP
	resp, _, err := client.Exchange(r, upstreamPrimary)
	if err != nil || resp == nil || len(resp.Answer) == 0 {
		log.Printf("Primary DNS (1.1.1.1) failed or returned no answer, forwarding to secondary DNS (%s)", upstreamSecondary)

		// If primary DNS failed or returned no valid record, try the secondary DNS (via UDP)
		client.Net = "udp" // Switch to UDP for the secondary server
		resp, _, err = client.Exchange(r, upstreamSecondary)
		if err != nil || resp == nil || len(resp.Answer) == 0 {
			log.Printf("Secondary DNS (10.148.1.1) failed or returned no answer: %v", err)
			dns.HandleFailed(w, r)
			return
		}
	}

	w.WriteMsg(resp)
}

func main() {
	// Create a DNS server mux
	mux := dns.NewServeMux()

	// Register the forwarding handler for all zones
	mux.HandleFunc(".", forwardDNSRequest)

	// Start the DNS server on UDP (listens for queries on port 53)
	server := &dns.Server{
		Addr:    "127.1.1.53:53", // Listen on port 53 for DNS queries
		Net:     "udp",           // Use UDP for incoming requests
		Handler: mux,             // Use the mux as the handler
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
