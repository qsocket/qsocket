<div align="center">
  <img src=".github/img/banner.png">
  <br>
  <br>


  [![GitHub All Releases][release-img]][release]
  [![Build][workflow-img]][workflow]
  [![Issues][issues-img]][issues]
  ![Docker Pulls][docker-pulls]
  [![License: MIT][license-img]][license]
</div>

[go-report]: https://goreportcard.com/report/github.com/qsocket/qsocket
[go-report-img]: https://goreportcard.com/badge/github.com/qsocket/qsocket
[release]: https://github.com/qsocket/qsocket/releases
[release-img]: https://img.shields.io/github/v/release/qsocket/qsocket
[downloads]: https://github.com/qsocket/qsocket/releases
[downloads-img]: https://img.shields.io/github/downloads/qsocket/qsocket/total?logo=github
[issues]: https://github.com/qsocket/qsocket/issues
[issues-img]: https://img.shields.io/github/issues/qsocket/qsocket?color=red
[docker-pulls]: https://img.shields.io/docker/pulls/qsocket/qsocket?logo=docker&label=docker%20pulls
[license]: https://raw.githubusercontent.com/qsocket/qsocket/master/LICENSE
[license-img]: https://img.shields.io/github/license/qsocket/qsocket.svg
[google-cloud-shell]: https://console.cloud.google.com/cloudshell/open?git_repo=https://github.com/qsocket/qsocket&tutorial=README.md
[workflow-img]: https://github.com/qsocket/qsocket/actions/workflows/main.yml/badge.svg
[workflow]: https://github.com/qsocket/qsocket/actions/workflows/main.yml
[qsrn]: https://www.qsocket.io/qsrn/

QSocket is a cross-platform networking utility which reads and writes E2E encrypted data across systems using the QSocket relay network ([QSRN][qsrn]).
It allows redirecting fully interactive PTY sessions with reverse connections, effectively allowing remote access to systems, forwarding traffic, and transferring files to and from systems under NAT networks or firewalls.

> [!WARNING]  
> This tool is in its early alpha development stage, featuring experimental functionality that may lack backwards compatibility, and users are advised to exercise caution and not use it in production environments.

> [!CAUTION]
> Due to the changes to the relay protocol, clients starting from version `v0.0.8-beta` will not be compatible with the older versions. You can still access your older instances using previous client versions `(>= v.0.0.7-beta)` until October 23, 2024. **After October 23, 2024, legacy relay support will end, and all out-of-date QSocket instances will become inaccessible!**



## Installation

[![Open in Cloud Shell](.github/img/cloud-shell.png)][google-cloud-shell]

|    **Tool**   |                 **Build From Source**                |       **Docker Image**      |                     **Binary Release**                    |
|:-------------:|:----------------------------------------------------:|:---------------------------:|:---------------------------------------------------------:|
| **qsock** | [Instructions](#building) | [Download](#docker-install) | [Download][release] |

---

QSocket supports 13 architectures and 12 operating systems, following table contains detailed list of all **Supported Platforms**. 

<details>
<summary>Supported Platforms</summary>

- ✅ `Supported`
- 🔧 `Supported, needs a cgo toolchain`
- 🚧 `In progress`
- ❌ `Unsupported`

| **ARCH/OS** | **Linux** | **MacOS** | **Windows** | **Android** | **iOS** | **FreeBSD** | **OpenBSD** | **NetBSD** | **Solaris** | **Illumos** | **Dragonfly** | **AIX** |
|:-----------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|:---------:|
| **AMD64** | ✅ | ✅ | ✅ | 🔧 | 🚧 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| **ARM64** | ✅ | ✅ | ✅ | ✅ | 🚧 | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **386** | ✅ | ❌ | ✅ | 🔧 | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **ARM32** | ✅ | ❌ | ❌ | 🔧 | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **RISCV64** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **LOONG64** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **MIPS32** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **MIPSLE** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **MIPS64** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **MIPS64LE** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **PPC64** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **PPC64LE** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **S390X** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

</details>


### Building

Building from source needs nothing but the Go toolchain, the default goal
builds the binary of the host platform into `./build`:

```bash
git clone https://github.com/qsocket/qsocket
cd qsocket
make
```

Every port of the toolchain this source tree supports is built by `make all`,
a single one of them by its `<os>/<arch>` name. The binaries are static, carry
no debug information and land in `build/<os>/qsock-<arch>`:

```bash
make all             # every target, -j builds them in parallel
make linux/arm64     # a single target
make linux           # every port of one operating system
make targets         # the target list
```

The android ports needing an external linker are built by `make android-ndk`
with the `ANDROID_NDK` variable pointing at a native development kit, the ios
ports by `make ios` with a cgo compiler wrapping the xcode toolchain.

### Release Archives

Every commit pushed to `master` is built for all of the targets by the
[build workflow][workflow], which uploads the binary of each one of them as a
separate artifact. `mkrelease.sh` fetches the artifacts of a build with the
[gh CLI](https://cli.github.com) and packages them into
`release/qsock_<os>_<arch>.tar.gz`:

```bash
./mkrelease.sh              # the latest build of the master branch
./mkrelease.sh -r <run-id>  # a build run of its own
./mkrelease.sh -h           # every option
```

### Docker Install

[![Docker](http://dockeri.co/image/qsocket/qsocket)](https://hub.docker.com/r/qsocket/qsocket/)

```bash
docker pull qsocket/qsocket:latest
docker run -it qsocket --help
```

## Usage

```
Usage: qsock <command> [flags]

Reads and writes E2E encrypted data across systems using the QSocket relay
network. (QSRN)

Flags:
  -h, --help                    Show context-sensitive help.
      --cert-fp=HEX             Hex encoded TLS certificate fingerprint for
                                validation.
      --relay-server=IP:PORT    Custom QSRN relay server address.
                                (default:relay.qsocket.io)
      --relay-sni=HOST          Spoof the TLS server name indicated to the relay
                                server.
  -n, --probe=5                 Probe interval for connecting QSRN.
  -C, --plain                   Disable all encryption.
      --e2e                     Use E2E encryption. (default:true)
  -l, --listen                  Server mode. (listen for connections)
  -g, --generate                Generate a new address. (random)
  -K, --pin                     Enable certificate pinning on TLS connections.
  -q, --quiet                   Quiet mode. (no stdout)
  -T, --tor                     Use TOR for connecting QSRN.
      --qr                      Generate a QR code with given stdin and print on
                                the terminal.
  -v, --verbose                 Verbose mode.
      --version                 Print the version and quit.

Commands:
  ssh [<uri> [<command> ...]] [flags]
    Start an interactive session on the remote peer.

  scp [<source> [<destination>]] [flags]
    Copy files and folders to/from the remote peer.

  netcat [<uri> [<command> ...]] [flags]
    Open a raw data channel to the remote peer.

Run "qsock <command> --help" for more information on a command.
```

The global flags are passed **before** the subcommand, each subcommand takes
its own options after it.

### Secret URIs

Secrets are written as a `qs://` URI and carry a UUID:

```bash
qsock -g
qs://f81d4fae-7dec-41d0-a765-00a0c91e6bf6
```

A secret which is not a UUID is hashed into one, so the secrets generated by
the previous releases stay usable with the new URI format:

```bash
qsock ssh qs://ElDdawDNV4Quz5SolGWT
```

The `qs://` scheme is optional, a value carrying no scheme is taken as a plain
ASCII secret and converted into its URI form, which makes the two invocations
below equivalent:

```bash
qsock ssh MySecret
qsock ssh qs://MySecret
```

A mistyped scheme (`qs:/MySecret`, `http://MySecret`) is rejected rather than
hashed into a secret of its own. The one place the scheme stays mandatory is
an `scp` operand, where it is what tells the remote end of the transfer apart
from the local path.

Both peers derive the same identifier from the same secret without exchanging
anything: the first 16 bytes of its SHA-256 digest, tagged as a version 8
UUID. Pick a strong secret, or let `-g` generate one.

### Server Mode

A listening peer serves whichever session the connecting peer asks for, so it
needs nothing but the secret:

```bash
qsock -l qs://<secret>
```

A command given in server mode becomes the default for the sessions arriving
without one, which is how a specific shell is pinned down:

```bash
qsock -l ssh qs://<secret> /bin/zsh
```

### Relay Server

Both peers reach each other through the public QSRN gate, `relay.qsocket.io`
on port 443. A gate of your own is dialed with `--relay-server`, the port is
optional and defaults to 443, or to 80 with `-C`:

```bash
qsock --relay-server gate.example.com ssh qs://<secret>
qsock --relay-server 10.0.0.1:1337 -l qs://<secret>
```

Both peers need to be on the same gate to find each other, a custom gate is
also dialed through the proxy when it is combined with `-T`.

### TLS Server Name (SNI)

The server name indicated to the gate during the TLS handshake is the name of
the gate itself, `--relay-sni` puts an arbitrary one on the wire instead:

```bash
qsock --relay-server gate.example.com --relay-sni www.google.com ssh qs://<secret>
qsock --relay-server 10.0.0.1:443 --relay-sni cdn.example.com -l qs://<secret>
```

This hides which host the connection is destined for from everything watching
the handshake, the name is the only part of it which travels in the clear. The
certificate presented by the gate still has to be valid for the name that was
indicated, so a spoofed name needs to be one the gate (or the CDN in front of
it) is able to answer for. A gate given as an IP address indicates no server
name at all, which is the default behaviour of a TLS client.

A session with the name spoofed on both ends, through a gate of your own which
is fronted by a CDN answering for the indicated names:

```bash
qsock --relay-server gate.example.com --relay-sni cdn.example.com -l qs://<secret>       # Workstation A
qsock --relay-server gate.example.com --relay-sni assets.example.com ssh qs://<secret>   # Workstation B
```

Each peer indicates the name to the gate it dials itself, so the two sides only
have to agree on the gate, not on the name. The public gate presents the
certificate of its own name and nothing else, so indicating an unrelated name
to it is refused while the handshake is still going on:

```bash
$ qsock --relay-sni www.google.com -l qs://<secret>
[ERROR] remote error: tls: handshake failure
```

Dropping the TLS layer with `-C` takes the server name off the wire entirely,
there is no handshake left to read it from, but it also leaves the session
travelling in the clear.

### ssh

**Usage:** `qsock ssh [options] qs://<secret> [command]`

Opens an interactive PTY session, exactly like a regular ssh client:

```bash
qsock ssh qs://<secret>
```

A command after the secret is executed by the shell of the remote peer instead
of starting a shell:

```bash
qsock ssh qs://<secret> app.exe
qsock ssh qs://<secret> "uname -a; id"
```

#### Port Forwarding

Local port forwarding is configured with `-L`, just as with a normal ssh
client:

```bash
qsock ssh -L 80:intra.example.com:80 qs://<secret>
```

This forwards any connection to port 80 on the local machine to port 80 on
intra.example.com, reached from the remote peer.

By default anyone, even on a different machine, can connect to the forwarded
port. Supplying a bind address restricts it to the local host:

```bash
qsock ssh -L 127.0.0.1:80:intra.example.com:80 qs://<secret>
```

A forwarding session never starts a shell, it only tunnels TCP traffic until
it is interrupted. `-L` is repeatable, so a single session can carry as many
forwarded ports as needed:

```bash
qsock ssh -L 2222:localhost:22 -L 8080:intra.example.com:80 qs://<secret>
```

`-N` is accepted for ssh compatibility and implied by `-L`.

#### SOCKS Proxy

> [!NOTE]
> `-D <port>` is parsed but not implemented yet, this feature is still in
> development.

### scp

**Usage:** `qsock scp [options] <source> <destination>`

Copies files and folders across devices, just like the classic scp utility.
Exactly one of the two operands carries a `qs://<secret>:/path` target, which
picks the direction of the transfer.

Copy a local file into the `target` directory of the remote peer:

```bash
qsock scp local.txt qs://<secret>:/target/
```

Copy a remote file into the current local directory:

```bash
qsock scp qs://<secret>:/target/remote.txt .
```

Folders need the `-r` flag:

```bash
qsock scp -r qs://<secret>:/target .
```

The transfer draws a progress bar for every file on its way.

### netcat

**Usage:** `qsock netcat [options] qs://<secret> [command]`

Same as `ssh`, except that it provides a raw data channel instead of an
interactive PTY, which makes it usable as a shell pipe:

```bash
qsock -l qs://<secret> > warez.tar.gz      # Workstation A
qsock netcat qs://<secret> < warez.tar.gz  # Workstation B
```

### Examples

- Log in to Workstation A from Workstation B through any firewall/NAT
```bash
qsock -l qs://<secret>        # Workstation A
qsock ssh qs://<secret>       # Workstation B
```

- Copy a folder from Workstation A to Workstation B
```bash
qsock -l qs://<secret>                      # Workstation A
qsock scp -r qs://<secret>:/etc/nginx .     # Workstation B
```

- SSH from *Workstation B* to *Workstation A* by port forwarding through any firewall/NAT
```bash
qsock -l qs://<secret>                            # Workstation A
qsock ssh -N -L 2222:localhost:22 qs://<secret>   # Workstation B
ssh -p 2222 user@localhost                        # Workstation B
```

- Access 192.168.6.7:80 on Workstation A's private LAN from Workstation B
```bash
qsock -l qs://<secret>                              # Workstation A
qsock ssh -N -L 8080:192.168.6.7:80 qs://<secret>   # Workstation B
curl http://127.0.0.1:8080                          # Workstation B
```

- Execute any command (nc -e style) on *Workstation A*
```bash
qsock -l qs://<secret>                              # Workstation A
qsock netcat qs://<secret> "echo hello_world; id"   # Workstation B
```

- Access the entirety of Workstation A's private LAN (Socks4/4a/5 proxy)
```bash
qsock -l qs://<secret>                            # Workstation A
qsock ssh -N -L 2222:localhost:22 qs://<secret>   # Workstation B
ssh -p 2222 -D 9090 root@localhost                # Workstation B
# Access www.google.com via Workstation A's private LAN from your Workstation B:
curl --socks4a 127.1:9090 http://www.google.com
```

- Mount a remote folder of Workstation A using sshfs and qsock
```bash
qsock -l qs://<secret>                            # Workstation A
qsock ssh -N -L 2222:localhost:22 qs://<secret>   # Workstation B
sudo sshfs -p 2222 -o allow_other,default_permissions root@localhost:/remote_dir /mnt/local_dir # Workstation B
```

- Relay the session through a private gate, indicating an unrelated server name
```bash
qsock --relay-server gate.example.com --relay-sni www.google.com -l qs://<secret>   # Workstation A
qsock --relay-server gate.example.com --relay-sni www.google.com ssh qs://<secret>  # Workstation B
```

---

**Crypto / Security Mumble Jumble**
- The connections are end-2-end encrypted. This means from User-2-User (and not just to the Relay Network). The Relay Network relays only (encrypted) data to and from the Users.
- The QSocket uses [SRP](https://en.wikipedia.org/wiki/Secure_Remote_Password_protocol) for ensuring [perfect forward secrecy](https://en.wikipedia.org/wiki/Forward_secrecy). This means that the session keys are always different, and recorded session traffic cannot be decrypted by the third parties even if the user secret is known.
- The session key is 256 bit and ephemeral. It is freshly generated for every session and generated randomly (and is not based on the password).
- A brute force attack against weak secrets requires a new TCP connection for every guess. But QSRN contains a strong load balancer which is limiting the consecutive connection attempts.
- Do not use stupid passwords like 'password123'. Malice might pick the same (stupid) password by chance and connect. If in doubt use *qsock -g* to generate a strong one. Alice's and Bob's password should at least be strong enough so that Malice can not guess it by chance while Alice is waiting for Bob to connect.
- If Alice shares the same password with Bob and Charlie and either one of them connects then Alice can not tell if it is Bob or Charlie who connected.
- Assume Alice shares the same password with Bob and Malice. When Alice stops listening for a connection then Malice could start to listen for the connection instead. Bob (when opening a new connection) can not tell if he is connecting to Alice or to Malice.
- We did not invent SRP. It's a well-known protocol, and it is well-analyzed and trusted by the community. 


https://user-images.githubusercontent.com/17179401/224060762-e0f121f6-431b-4eb5-8833-4a5d533003de.mp4

---

<details>
<summary>RDP connection over QSRN</summary>

https://github.com/qsocket/qs-netcat/assets/17179401/af46c8fb-cb33-483a-b5c1-9142843da2bd

</details>


<details>
<summary>ADB access over QSRN</summary>

https://user-images.githubusercontent.com/17179401/216651601-6ddc8ddf-7248-4c2b-bd77-00f00f773c80.mov
    
</details>
