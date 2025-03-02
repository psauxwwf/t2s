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

socks:
  host: "127.0.0.1"
  port: 9050

dns:
  listen: "127.1.1.53"
  render: true
  resolvers:
    - "127.1.1.253:53/tcp"
    - "1.1.1.1:53/tcp"
```

---

### Repait systemd-resolved if symlink deleted

```bash
sudo ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
```

---

- https://github.com/xjasonlyu/tun2socks/wiki/Proxy-Models
