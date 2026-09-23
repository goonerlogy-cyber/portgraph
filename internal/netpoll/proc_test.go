package netpoll

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeHexAddr(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		ip   string
		port uint16
	}{
		{"IPv4 loopback", "0100007F:1F90", "127.0.0.1", 8080},
		{"IPv4 wildcard", "00000000:0000", "0.0.0.0", 0},
		{"IPv4 public", "08080808:0035", "8.8.8.8", 53},
		{"IPv6 loopback", "00000000000000000000000001000000:1F90", "::1", 8080},
		{"IPv6 wildcard", "00000000000000000000000000000000:0000", "::", 0},
		{"IPv6 global", "B80D012078563412F0DEBC9A01000000:01BB", "2001:db8:1234:5678:9abc:def0:0:1", 443},
		{"IPv6 link local", "000080FE000000000000000001000000:FFFF", "fe80::1", 65535},
		{"IPv4 mapped", "0000000000000000FFFF0000010200C0:0035", "::ffff:192.0.2.1", 53},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := decodeHexAddr(nativeAddr(t, tt.hex))
			if err != nil {
				t.Fatalf("decodeHexAddr: %v", err)
			}
			if ip != tt.ip || port != tt.port {
				t.Fatalf("got %s port %d, want %s port %d", ip, port, tt.ip, tt.port)
			}
		})
	}
}

func TestDecodeHexAddrRejectsMalformedInput(t *testing.T) {
	for _, addr := range []string{
		"", "0100007F", "0100007F:1F90:extra", "0100007:1F90",
		"000000000000000000000000:1F90", "GG00007F:1F90",
		"000000000000000000000000GG000000:1F90",
		"0100007F:10000", "0100007F:ZZZZ", "0100007F:", "0100007F:1",
	} {
		t.Run(addr, func(t *testing.T) {
			if _, _, err := decodeHexAddr(addr); err == nil {
				t.Fatalf("expected error for %q", addr)
			}
		})
	}
}

func TestParseProcNetFile(t *testing.T) {
	for _, proto := range []Proto{TCP, UDP} {
		t.Run(string(proto), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), string(proto))
			local := nativeAddr(t, "00000000000000000000000001000000:1F90")
			remote := nativeAddr(t, "B80D0120000000000000000002000000:01BB")
			contents := procHeader + "\n\nmalformed\n" +
				procRow("invalid", remote, "01", "1") +
				procRow(local, "invalid", "01", "2") +
				procRow(local, remote, "01", "bad-inode") +
				procRow(local, remote, "01", "42")
			writeProcFile(t, path, contents)
			conns, err := parseProcNetFile(path, proto)
			if err != nil {
				t.Fatalf("parseProcNetFile: %v", err)
			}
			state := "ESTABLISHED"
			if proto == UDP {
				state = "OPEN"
			}
			want := []Conn{{Proto: proto, LocalIP: "::1", LocalPort: 8080,
				RemoteIP: "2001:db8::2", RemotePort: 443, State: state, Inode: 42}}
			if !reflect.DeepEqual(conns, want) {
				t.Fatalf("got %+v, want %+v", conns, want)
			}
		})
	}
}

func TestParseProcNetFileReportsReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tcp6")
	writeProcFile(t, path, procHeader+"\n"+strings.Repeat("x", 128*1024))
	if _, err := parseProcNetFile(path, TCP); err == nil {
		t.Fatal("expected scanner error for oversized record")
	}
}

func TestPollNetFiles(t *testing.T) {
	dir := t.TempDir()
	ipv4 := nativeAddr(t, "0100007F:1F90")
	ipv6 := nativeAddr(t, "00000000000000000000000001000000:1F90")
	for _, table := range []struct {
		name string
		addr string
	}{
		{"tcp", ipv4}, {"udp", ipv4}, {"tcp6", ipv6}, {"udp6", ipv6},
	} {
		writeProcFile(t, filepath.Join(dir, table.name),
			procHeader+"\n"+procRow(table.addr, table.addr, "01", "42"))
	}
	conns, err := pollNetFiles(dir)
	if err != nil {
		t.Fatalf("pollNetFiles: %v", err)
	}
	if len(conns) != 4 {
		t.Fatalf("got %d connections, want 4", len(conns))
	}
	for i, want := range []struct {
		proto Proto
		ip    string
	}{{TCP, "127.0.0.1"}, {UDP, "127.0.0.1"}, {TCP, "::1"}, {UDP, "::1"}} {
		if conns[i].Proto != want.proto || conns[i].LocalIP != want.ip {
			t.Errorf("connection %d = %+v, want %s %s", i, conns[i], want.proto, want.ip)
		}
	}
}

func TestPollNetFilesWithoutIPv6(t *testing.T) {
	dir := t.TempDir()
	addr := nativeAddr(t, "0100007F:1F90")
	writeProcFile(t, filepath.Join(dir, "tcp"), procHeader+"\n"+procRow(addr, addr, "0A", "42"))
	writeProcFile(t, filepath.Join(dir, "udp"), procHeader+"\n")
	conns, err := pollNetFiles(dir)
	if err != nil || len(conns) != 1 || conns[0].State != "LISTEN" {
		t.Fatalf("got %+v, %v; want IPv4 listener without error", conns, err)
	}
}

func TestPollNetFilesRetainsConnectionsOnError(t *testing.T) {
	dir := t.TempDir()
	addr := nativeAddr(t, "00000000000000000000000001000000:1F90")
	writeProcFile(t, filepath.Join(dir, "tcp6"), procHeader+"\n"+procRow(addr, addr, "01", "42"))
	// Existing but unreadable IPv6 tables must not be mistaken for disabled IPv6.
	if err := os.Mkdir(filepath.Join(dir, "udp6"), 0700); err != nil {
		t.Fatal(err)
	}
	conns, err := pollNetFiles(dir)
	if len(conns) != 1 || conns[0].LocalIP != "::1" {
		t.Fatalf("lost connection from readable table: %+v", conns)
	}
	if err == nil {
		t.Fatal("expected errors from missing IPv4 and unreadable IPv6 tables")
	}
	for _, table := range []string{"tcp", "udp", "udp6"} {
		if !strings.Contains(err.Error(), "read "+table+" sockets") {
			t.Errorf("error %q does not identify %s", err, table)
		}
	}
}

func TestConnKey(t *testing.T) {
	for _, tt := range []struct {
		local, remote, want string
	}{
		{"127.0.0.1", "192.0.2.1", "tcp:127.0.0.1:8080-192.0.2.1:443"},
		{"::1", "2001:db8::2", "tcp:[::1]:8080-[2001:db8::2]:443"},
	} {
		conn := Conn{Proto: TCP, LocalIP: tt.local, LocalPort: 8080, RemoteIP: tt.remote, RemotePort: 443}
		if got := conn.Key(); got != tt.want {
			t.Errorf("Key() = %q, want %q", got, tt.want)
		}
	}
}

const procHeader = "sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode"

func procRow(local, remote, state, inode string) string {
	return fmt.Sprintf("0: %s %s %s 00000000:00000000 00:00000000 00000000 1000 0 %s 1\n",
		local, remote, state, inode)
}

func writeProcFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

// The fixtures use the little-endian spelling common on x86 Linux. Adapt
// each word to the host so these tests also cover native big-endian builds.
func nativeAddr(t *testing.T, addr string) string {
	t.Helper()
	parts := strings.Split(addr, ":")
	raw, err := hex.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var result strings.Builder
	for i := 0; i < len(raw); i += 4 {
		var word [4]byte
		binary.LittleEndian.PutUint32(word[:], binary.BigEndian.Uint32(raw[i:i+4]))
		fmt.Fprintf(&result, "%08X", binary.NativeEndian.Uint32(word[:]))
	}
	return result.String() + ":" + parts[1]
}
