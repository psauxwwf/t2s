# t2s

Route host traffic through tun2socks with pluggable backends.

Supported backends:

- `socks` (`socks5`, `ss`, `relay`)
- `ssh`
- `chisel`
- `dnstt`
- `freeturn`

Also supports Tor via local SOCKS endpoint (`proxy.type: socks`).

## Commands

Run app (default action):

```bash
sudo t2s run
# same as
sudo t2s
```

Generate default config:

```bash
t2s save
```

Repair resolver state:

```bash
sudo t2s repair
```

Default config path: `~/.config/t2s/config.yaml`.

You can override config path with `--config <path>` for any command.

Install/uninstall systemd integration:

```bash
sudo t2s service install
sudo t2s service uninstall
```

`service uninstall` stops/disables service and removes unit file. Binary `/usr/local/bin/t2s` is kept.

## Flags

Global flags:

- `--config <path>` path to config yaml
- `--level <debug|info|warn|error>` log level
- `--log-file <path>` if set, JSON logs go to this file; otherwise logs are text to stderr only

Run-only flag:

- `t2s run --timeout <sec>` stop after timeout (`0` means no timeout)

## Config examples

### SOCKS5

```yaml
proxy:
  type: socks

interface:
  device: tun0
  exclude:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 185.62.202.0/24
    - 185.62.200.0/23
  custom_routes: []
  metric: 512
  sleep: 0

socks:
  proto: socks5
  username: username
  password: password
  host: 1.3.3.7
  port: 1080
  args: ""

dns:
  enable: true
  listen: 127.1.1.53
  render: true
  resolvectl: true
  resolvers:
    - ip: 1.1.1.1
      proto: tcp
      port: 53
      rule: ".*"
  records: {}
```

### Shadowsocks (`proto: ss`)

```yaml
proxy:
  type: socks
socks:
  proto: ss
  username: AEAD_CHACHA20_POLY1305
  password: password
  host: 1.3.3.7
  port: 1080
  args: ""
```

### GOST relay (`proto: relay`)

```yaml
proxy:
  type: socks
socks:
  proto: relay
  username: username
  password: password
  host: 1.3.3.7
  port: 1080
  args: "nodelay=true"
```

### SSH

```yaml
proxy:
  type: ssh
ssh:
  username: user
  host: ssh.host.com
  port: 22
  args:
    - -o
    - ProxyCommand=cloudflared access ssh --hostname %h
```

### Chisel

```yaml
proxy:
  type: chisel
chisel:
  server: https://chisel.domain.xyz
  username: username
  password: password
  proxy: ""
```

Chisel via proxy:

```yaml
proxy:
  type: chisel
interface:
  device: tun0
  exclude:
    - 1.3.3.7
chisel:
  server: https://chisel.domain.xyz
  username: username
  password: password
  proxy: socks5h://proxy_username:proxy_password@1.3.3.7:1080
```

### DNSTT

```yaml
proxy:
  type: dnstt
dnstt:
  resolver: udp://8.8.8.8:53
  # resolver: dot://8.8.8.8:853
  # resolver: https://dns.google/dns-query
  # resolver: udp://77.88.8.8:53
  # resolver: dot://77.88.8.8:853
  # resolver: https://common.dot.dns.yandex.net/dns-query
  pubkey: "7c25844f2536a3d82b9a7a4c052f119f34ec97919bf9574679897d08f241ca48"
  domain: t.domain.xyz
  username: username
  password: password
```

### Free TURN Proxy

This backend runs the upstream `free-turn-proxy` Go client in-process through
its public `mobile` API and exposes it to tun2socks as
`socks5://127.0.0.1:<dynamic-port>`.

```yaml
proxy:
  type: freeturn
freeturn:
  peer: 203.0.113.10:56002
  links:
    - https://vk.ru/call/join/example-1
    - https://vk.ru/call/join/example-2
  obf_profile: rtpopus
  obf_key: 61caefa65c98d428b426d40c75251d77d96a46c2c8962284168704ba37ef6ef5
  manual_captcha: false
```

`-mode tcp` is fixed and is not exposed in YAML. `manual_captcha` is a boolean
and defaults to `false` when omitted.

The in-process free-turn TCP client is a raw TCP forwarder. For whole-host t2s
routing, run the remote free-turn server with a TCP `-connect` target that
speaks SOCKS5.
If the client discovers TURN/server IPs dynamically, add those routes through
the original gateway with `interface.custom_routes` to avoid tunnel loops.

### Tor (via local SOCKS)

```yaml
proxy:
  type: socks
socks:
  proto: socks5
  host: 127.0.0.1
  port: 9050
  username: ""
  password: ""
  args: ""
```

## DNS snippets

Custom records:

```yaml
dns:
  listen: 127.1.1.53
  render: true
  resolvectl: true
  resolvers:
    - ip: 1.1.1.1
      port: 53
      proto: tcp
      rule: ".*"
  records:
    test01.lan: 10.10.10.1
    test02.lan: 10.10.10.2
```

Rule-based resolver (leak control):

```yaml
dns:
  resolvers:
    - ip: 1.1.1.1
      port: 53
      proto: tcp
      rule: ".*"
    - ip: 10.10.10.10
      port: 53
      proto: udp
      rule: ".*github\\.com"
```

### If you run via ssh - must add exclude to your ssh connect address

```bash
ss -tunp | grep ssh
tcp    ESTAB   0        36         1.3.3.9:56777     1.3.3.8:47284   users:(("sshd",pid=1627,fd=4))
tcp    ESTAB   0        0          1.3.3.9:56777     1.3.3.8:47152   users:(("sshd",pid=1278,fd=4))
```

```yaml
interface:
  device: "tun0"
  exclude:
    - "1.3.3.8"
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"
    - "185.62.202.0/24"
    - "185.62.200.0/23"
```

## References

- [Proxy models for tun2socks](https://github.com/xjasonlyu/tun2socks/wiki/Proxy-Models)
- [dnstt](https://dnstt.network/)
- [gost](https://github.com/go-gost/gost)
- [chisel](https://github.com/jpillora/chisel)
- [free-turn-proxy](https://github.com/samosvalishe/free-turn-proxy)
