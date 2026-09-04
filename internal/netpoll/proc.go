package netpoll

import (
	"bufio"
	"fmt"
	"os"
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
	return fmt.Sprintf("%s:%s:%d-%s:%d", c.Proto, c.LocalIP, c.LocalPort, c.RemoteIP, c.RemotePort)
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
	var conns []Conn

	tcp, err := parseProcNetFile("/proc/net/tcp", TCP)
	if err == nil {
		conns = append(conns, tcp...)
	}

	udp, err := parseProcNetFile("/proc/net/udp", UDP)
	if err == nil {
		conns = append(conns, udp...)
	}

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

	return conns, nil
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
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad addr %q", s)
	}

	ipHex := parts[0]
	if len(ipHex) != 8 {
		return "", 0, fmt.Errorf("only ipv4 supported: %q", s)
	}

	var b [4]byte
	for i := range 4 {
		v, err := strconv.ParseUint(ipHex[i*2:i*2+2], 16, 8)
		if err != nil {
			return "", 0, err
		}
		b[i] = byte(v)
	}
	ip := fmt.Sprintf("%d.%d.%d.%d", b[3], b[2], b[1], b[0])

	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, err
	}

	return ip, uint16(port), nil
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
