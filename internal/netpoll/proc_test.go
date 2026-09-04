package netpoll

import "testing"

func TestDecodeHexAddr(t *testing.T) {
	ip, port, err := decodeHexAddr("0100007F:1F90")
	if err != nil {
		t.Fatalf("decodeHexAddr: %v", err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", ip)
	}
	if port != 8080 {
		t.Fatalf("expected port 8080, got %d", port)
	}
}

func TestDecodeHexAddrRejectsIPv6(t *testing.T) {
	_, _, err := decodeHexAddr("00000000000000000000000001000000:1F90")
	if err == nil {
		t.Fatalf("expected error for non-ipv4 address")
	}
}

func TestParseProcNetFile(t *testing.T) {
	conns, err := parseProcNetFile("/proc/net/tcp", TCP)
	if err != nil {
		t.Fatalf("parseProcNetFile: %v", err)
	}
	for _, c := range conns {
		if c.LocalPort == 0 && c.RemotePort == 0 && c.LocalIP == "" {
			t.Fatalf("parsed a bogus empty connection: %+v", c)
		}
	}
}
