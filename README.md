# portgraph

Live TCP/UDP connections rendered as an animated force-directed graph in
your terminal, using Unicode braille characters for 2x4 sub-cell resolution
per terminal cell.

Reads `/proc/net/tcp` and `/proc/net/udp` directly (Linux only), so it only
needs to run as your own user to see your own process's connections. Seeing
every connection on the box may need root, depending on kernel hardening.
IPv4 only for now, IPv6 (`/proc/net/tcp6`, `/proc/net/udp6`) isn't parsed.

## Install

```
go install github.com/arshnah/portgraph@latest
```

Or build from source:

```
git clone https://github.com/arshnah/portgraph
cd portgraph
go build -o portgraph .
```

## Usage

```
portgraph
```

Every 500ms it re-reads `/proc/net/*`, maps each connection's inode back to
an owning process by walking `/proc/*/fd`, and feeds the result into a
Fruchterman-Reingold force-directed layout: one node per process, one node
per remote host, edges between them, nodes repel each other, edges pull
their endpoints together. Blue dots are TCP, orange dots are UDP. The
footer lists the top 10 busiest processes by connection count.

`q` / `ctrl+c` to quit.

## License

See [LICENSE](LICENSE).
