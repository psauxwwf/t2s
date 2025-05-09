### Proxy to socks5

```yaml
proxy:
  type: "socks" # socks/ssh
---
socks:
  proto: "socks5" # socks5/ss/relay
  username: "username"
  password: "password"
  host: "100.3.3.7"
  port: 1080
```

### Proxy to SSH

```bash
vim /etc/ssh/sshd_config
```

```bash
AllowTcpForwarding yes
```

```yaml
proxy:
  type: "ssh" # socks/ssh
---
ssh:
  username: "user"
  host: "100.3.3.7"
  port: 1337
  extra: ""
```

### Proxy to [SS](https://github.com/shadowsocks/go-shadowsocks2)

> [!note] With obfuscating
> https://github.com/shadowsocks/simple-obfs > https://github.com/shadowsocks/v2ray-plugin

```bash
wget https://github.com/shadowsocks/go-shadowsocks2/releases/download/v0.1.5/shadowsocks2-linux.tgz
tar -xf shadowsocks2-linux.tgz
shadowsocks2-linux -s 'ss://AEAD_CHACHA20_POLY1305:password@100.3.3.7:1080' -verbose
```

```yaml
proxy:
  type: "socks" # socks/ssh
---
socks:
  proto: "ss"
  username: "AEAD_CHACHA20_POLY1305"
  password: "password"
  host: "100.3.3.7"
  port: 1080
  extra: ""
```

```
ss://method:password@server_host:port/<?obfs=http;obfs-host=xxx>
```

### Proxy to [GOST](https://github.com/go-gost/gost)

```bash
wget https://github.com/go-gost/gost/releases/download/v3.0.0/gost_3.0.0_linux_amd64.tar.gz
tar -xf gost_3.0.0_linux_amd64.tar.gz
gost -L=relay://username:password@100.3.3.7:1080
```

```yaml
proxy:
  type: "socks" # socks/ssh
---
socks:
  proto: "relay" # socks5/ss/relay
  username: "username"
  password: "password"
  host: "100.3.3.7"
  port: 1080
  extra: ""
```

```
relay://<username>:<password>@server_host:port?<nodelay=false>
```

### Proxy to tor

[Get bridges](https://bridges.torproject.org/bridges?transport=obfs4)

```bash
sudo apt-get install tor obfs4proxy --yes
```

```bash
vim /etc/tor/torrc
```

```yaml
SocksPort 127.0.0.1:1080
DNSPort 127.1.2.53:53

UseBridges 1
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy

Bridge obfs4 212.21.66.73:35401 C36F8D3C481910ED7A34F5ECEBE1C7C9A258F4A8 cert=9IygPQi2UKJ6pUjYTHl8ltg1cuPDvcsE9Os9TPVSioR0qmXU/0uSvD3rsm3jskV1nupJAg iat-mode=2
Bridge obfs4 140.186.139.199:4444 A17E0D3FE22FA225EB4ACFEF242DBD1C71ED1D6B cert=4xg9Uri1mhV9PaHX7J4Uc2y/6VLdSiwJO8TQFDE8g0f0M1hGjQYfkO39h+sIw+L3vR1IeQ iat-mode=0
Bridge obfs4 141.95.106.45:12558 F1A7BBDED674C0654B04ED387FFCB1A5DD2B2ED5 cert=TWRS4j6AKbKH/SL/bAqHkP7fI7C3P3dQoV+D8pRgqcJCK+r4SvZhg3k661ikgg732nuADA iat-mode=0
Bridge obfs4 54.38.138.85:21641 E8D24300464D24AB6D905B3D01029E010363D731 cert=g7Gsuzkk2ZG88oslXKYx/Cn1XHj3DaAJRKARzN1kHrfa4B4mTCjF/0v+d1HxUr4ujYvXCQ iat-mode=0
```

_Check_

```bash
tor --verify-config
systemctl enable tor --now
systemctl restart tor
curl --socks5-hostname 127.0.0.1:1080 http://wiki47qqn6tey4id7xeqb6l7uj6jueacxlqtk3adshox3zdohvo35vad.onion
curl --socks5-hostname 127.0.0.1:1080 ident.me
```

```bash
vim config.yaml
```

```yaml
proxy:
  type: "socks"

interface:
  device: "tun0"
  exclude:
    - "54.38.138.85"
    - "140.186.139.199"
    - "141.95.106.45"
    - "212.21.66.73"
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"
  metric: 512
  sleep: 5

socks:
  host: "127.0.0.1"
  port: 9050

dns:
  listen: "127.1.1.53"
  render: true
  resolvers:
    - ip: "127.1.2.53"
      port: 53
      proto: tcp
      rule: ""
    - ip: "1.1.1.1"
      port: 53
      proto: tcp
      rule: ""
```

### Proxy to [Chisel](https://github.com/jpillora/chisel)

```bash
wget https://github.com/jpillora/chisel/releases/download/v1.10.1/chisel_1.10.1_linux_amd64.gz
gunzip chisel_1.10.1_linux_amd64.gz
chmod +x chisel_1.10.1_linux_amd64
openssl req -x509 -nodes -newkey rsa:2048 -keyout server.key -out server.crt -days 365
./chisel_1.10.1_linux_amd64 server -p 443 --auth username:password --tls-cert server.crt --tls-key server.key --socks5
```

```yaml
proxy:
  type: "chisel"
---
chisel:
  server: "https://chisel.com"
  username: "username"
  password: "password"
  proxy: ""
```

### Proxy to Chisel via proxy

```yaml
proxy:
  type: "chisel"
interface:
  device: "tun0"
  exclude:
    - "1.3.3.7"
---
chisel:
  server: "https://chisel.com"
  username: "username"
  password: "password"
  proxy: "socks5h://proxy_username:proxy_password@1.3.3.7:1080" # only support http/socks5h/socks
```

### Proxy to ssh via Cloudfared

```yaml
proxy:
  type: ssh
interface:
  device: tun0
  exclude:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
  custom_routes:
    - 104.21.88.227/32 via 192.168.0.1 dev wlp3s0 # routes for cloudflare servers
    - 172.67.153.180/32 via 192.168.0.1 dev wlp3s0 # routes for cloudflare servers
  metric: 512
  sleep: 5 # sleep for connect to cloudflare
socks:
  proto: socks5
  username: ""
  password: ""
  host: 127.0.0.1
  port: 1080
  args: ""
ssh:
  username: "user"
  host: "ssh.host.com"
  port: 22
  args:
    - -o
    - ProxyCommand=cloudflared access ssh --hostname %h
chisel:
  server: ""
  username: ""
  password: ""
  proxy: ""
dns:
  listen: 127.1.1.53
  render: true
  resolvers:
    - ip: 1.1.1.1
      proto: tcp
      port: 53
      rule: ""
  records:
    ssh.host.com: 172.67.153.180 # lookup your host (same as cloudflare servers)
```

### Custom records for dns

```yaml
dns:
  listen: "127.1.1.53"
  render: true
  resolvers:
    - ip: "1.1.1.1"
      port: 53
      proto: tcp
      rule: ""
  records:
    test01.lan: "10.10.10.1"
    test02.lan: "10.10.10.2"
    test03.lan: "10.10.10.3"
    test04.lan: "10.10.10.4"
```

### Lock dns leak

> Local dns 10.10.10.10 support resolv only for github.com

```yaml
dns:
  listen: "127.1.1.53"
  render: true
  resolvers:
    - ip: "1.1.1.1"
      port: 53
      proto: tcp
      rule: ""
    - ip: "10.10.10.10"
      port: 53
      proto: udp
      rule: '.*github\.com'
```

### If you run via ssh - must add exclude to your ssh connect address

```bash
ss -tunp | grep ssh
tcp    ESTAB   0        36         85.239.54.158:56777     79.140.111.66:47284   users:(("sshd",pid=1627,fd=4))
tcp    ESTAB   0        0          85.239.54.158:56777     79.140.111.66:47152   users:(("sshd",pid=1278,fd=4))
```

```yaml
interface:
  device: "tun0"
  exclude:
    - "79.140.111.66"
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"
```

### Systemd unit

```ini
[Unit]
Description=t2s
After=network.target
Wants=network.target

[Service]
User=root
ExecStart=/usr/local/bin/t2s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.targe
```

---

### Repait systemd-resolved if symlink deleted

```bash
t2s -repair
```

or

```bash
for in in $(ip a | grep '^[0-9]:' | cut -d ':' -f 2 | tr -d ' ' | grep -v lo); do sudo resolvectl revert $in; done
sudo rm -f /etc/resolv.conf
sudo ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
sudo systemctl restart systemd-resolved
```

---

- https://github.com/xjasonlyu/tun2socks/wiki/Proxy-Models
