package netpoll

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Proto string

const (
	TCP Proto = "tcp"
	UDP Proto = "udp"
)

type Conn struct {
	Proto      Proto
	LocalIP    string
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
	State      string
	Inode      uint64
	PID        int
	Process    string
}

func (c Conn) Key() string {
	return fmt.Sprintf("%s:%s-%s", c.Proto,
		net.JoinHostPort(c.LocalIP, strconv.Itoa(int(c.LocalPort))),
		net.JoinHostPort(c.RemoteIP, strconv.Itoa(int(c.RemotePort))))
}

var tcpStates = map[string]string{
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

func Poll() ([]Conn, error) {
	conns, err := pollNetFiles("/proc/net")

	inodeToPID := buildInodePIDMap()
	pidToName := map[int]string{}

	for i := range conns {
		if pid, ok := inodeToPID[conns[i].Inode]; ok {
			conns[i].PID = pid
			name, ok := pidToName[pid]
			if !ok {
				name = readComm(pid)
				pidToName[pid] = name
			}
			conns[i].Process = name
		}
	}

	return conns, err
}

func pollNetFiles(dir string) ([]Conn, error) {
	tables := []struct {
		name     string
		proto    Proto
		optional bool
	}{
		{"tcp", TCP, false},
		{"udp", UDP, false},
		{"tcp6", TCP, true},
		{"udp6", UDP, true},
	}
	var conns []Conn
	var errs []error
	for _, table := range tables {
		entries, err := parseProcNetFile(filepath.Join(dir, table.name), table.proto)
		conns = append(conns, entries...)
		// Kernels without IPv6 support may omit both IPv6 tables.
		if err != nil && !(table.optional && errors.Is(err, os.ErrNotExist)) {
			errs = append(errs, fmt.Errorf("read %s sockets: %w", table.name, err))
		}
	}
	return conns, errors.Join(errs...)
}

func parseProcNetFile(path string, proto Proto) ([]Conn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []Conn
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		localIP, localPort, err := decodeHexAddr(fields[1])
		if err != nil {
			continue
		}
		remoteIP, remotePort, err := decodeHexAddr(fields[2])
		if err != nil {
			continue
		}

		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}

		state := fields[3]
		if proto == TCP {
			if name, ok := tcpStates[state]; ok {
				state = name
			}
		} else {
			state = "OPEN"
		}

		conns = append(conns, Conn{
			Proto:      proto,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      state,
			Inode:      inode,
		})
	}

	return conns, scanner.Err()
}

func decodeHexAddr(s string) (string, uint16, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[1]) != 4 {
		return "", 0, fmt.Errorf("bad addr %q", s)
	}

	ipHex := parts[0]
	if len(ipHex) != 8 && len(ipHex) != 32 {
		return "", 0, fmt.Errorf("bad IP address length: %q", s)
	}

	// Linux prints addresses as native-endian 32-bit words: one for IPv4,
	// four for IPv6. Reverse bytes within each word on little-endian hosts,
	// not the entire IPv6 address.
	var raw [16]byte
	for i := 0; i < len(ipHex); i += 8 {
		v, err := strconv.ParseUint(ipHex[i:i+8], 16, 32)
		if err != nil {
			return "", 0, err
		}
		binary.NativeEndian.PutUint32(raw[i/2:i/2+4], uint32(v))
	}
	ip, _ := netip.AddrFromSlice(raw[:len(ipHex)/2])

	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, err
	}

	return ip.String(), uint16(port), nil
}

func buildInodePIDMap() map[uint64]int {
	m := map[uint64]int{}

	procDir, err := os.Open("/proc")
	if err != nil {
		return m
	}
	defer procDir.Close()

	entries, err := procDir.ReadDir(-1)
	if err != nil {
		return m
	}

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(fdDir + "/" + fd.Name())
			if err != nil {
				continue
			}
			if !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inodeStr := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			inode, err := strconv.ParseUint(inodeStr, 10, 64)
			if err != nil {
				continue
			}
			m[inode] = pid
		}
	}

	return m
}

func readComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return fmt.Sprintf("pid %d", pid)
	}
	return strings.TrimSpace(string(data))
}
