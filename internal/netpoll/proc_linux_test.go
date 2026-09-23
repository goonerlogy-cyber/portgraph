package netpoll

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestPollFindsIPv6Sockets(t *testing.T) {
	if _, err := os.Stat("/proc/net/tcp6"); errors.Is(err, os.ErrNotExist) {
		t.Skip("kernel does not expose IPv6 socket tables")
	}
	tcp, err := net.ListenTCP("tcp6", &net.TCPAddr{IP: net.IPv6loopback})
	if errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EADDRNOTAVAIL) {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	udp, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()

	conns, err := Poll()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		proto Proto
		port  int
	}{
		{TCP, tcp.Addr().(*net.TCPAddr).Port},
		{UDP, udp.LocalAddr().(*net.UDPAddr).Port},
	} {
		found := false
		for _, conn := range conns {
			if conn.Proto != want.proto || conn.LocalIP != "::1" || int(conn.LocalPort) != want.port {
				continue
			}
			found = true
			if conn.PID != os.Getpid() || conn.Process == "" || conn.Inode == 0 {
				t.Errorf("IPv6 socket is missing its owner: %+v", conn)
			}
			break
		}
		if !found {
			t.Errorf("Poll did not find %s listener on [::1]:%d", want.proto, want.port)
		}
	}
}
