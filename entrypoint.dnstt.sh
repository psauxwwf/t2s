#!/bin/bash
mkdir -p db

if [[ ! -f db/server.key || ! -f db/server.pub ]]; then
	dnstt-server-linux-amd64 \
		-gen-key \
		-privkey-file db/server.key \
		-pubkey-file db/server.pub
fi

if [[ $# -eq 0 ]]; then
	exec dnstt-server-linux-amd64 \
		-udp "${DNS_PORT:-:53}" \
		-mtu "${MTU:-512}" \
		-privkey-file db/server.key \
		"$NS_QUERY" \
		"${UPSTREAM:-127.0.0.1:1080}"
else
	exec dnstt-server-linux-amd64 "$@"
fi
