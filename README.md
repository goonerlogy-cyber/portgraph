# portgraph

Live TCP/UDP connections rendered as an animated force-directed graph in
your terminal, using Unicode braille characters for 2x4 sub-cell resolution
per terminal cell.

Reads the IPv4 and IPv6 TCP/UDP tables in `/proc/net` directly (Linux only),
so it only needs to run as your own user to see your own process's
connections. Seeing every connection on the box may need root, depending
on kernel hardening. IPv6 sockets from `tcp6` and `udp6` use the same
process mapping and protocol colors as IPv4 sockets. Kernels without IPv6
support can omit these tables; errors reading other tables appear in the
footer while connections from readable tables remain visible.

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
