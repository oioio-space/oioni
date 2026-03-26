// system/wifi/ap_dhcp.go — minimal DHCP server for the AP subnet
//
// Implements DHCPv4 DISCOVER→OFFER and REQUEST→ACK over raw UDP broadcast.
// Leases are held in memory; clients must re-request after Pi reboot.
// Uses only stdlib — no external dependency required for this subset of DHCP.
package wifi

import (
	"context"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"syscall"
)

// apDHCPServer is a minimal DHCPv4 server for the AP subnet.
type apDHCPServer struct {
	iface  string
	gw     net.IP   // gateway = uap0 IP
	subnet net.IP   // network address
	mask   net.IPMask
	dns    []net.IP
	start  net.IP // first leasable IP (.100)
	end    net.IP // last leasable IP (.200)

	mu     sync.Mutex
	leases map[[6]byte]net.IP // MAC → assigned IP

	conn *net.UDPConn
	wg   sync.WaitGroup
}

// newAPDHCPServer creates an apDHCPServer from APConfig.
// cfg.IP must be a CIDR (e.g. "192.168.4.1/24").
func newAPDHCPServer(iface string, cfg APConfig) *apDHCPServer {
	ip, netw, err := net.ParseCIDR(cfg.IP)
	if err != nil {
		// Fallback to spec defaults on parse failure.
		ip = net.ParseIP("192.168.4.1")
		_, netw, _ = net.ParseCIDR("192.168.4.0/24")
	}
	gw := ip.To4()

	// Pool: subnet base + 100 to + 200
	base := netw.IP.To4()
	start := cloneIP(base)
	start[3] = 100
	end := cloneIP(base)
	end[3] = 200

	var dnsIPs []net.IP
	for _, d := range cfg.DNS {
		if ip := net.ParseIP(d); ip != nil {
			dnsIPs = append(dnsIPs, ip.To4())
		}
	}
	if len(dnsIPs) == 0 {
		dnsIPs = []net.IP{net.ParseIP("8.8.8.8").To4(), net.ParseIP("8.8.4.4").To4()}
	}

	return &apDHCPServer{
		iface:  iface,
		gw:     gw,
		subnet: base,
		mask:   netw.Mask,
		dns:    dnsIPs,
		start:  start,
		end:    end,
		leases: make(map[[6]byte]net.IP),
	}
}

// Start opens the DHCP socket bound to s.iface only and begins serving.
// SO_BINDTODEVICE restricts the socket to uap0 so it does not conflict
// with udhcpd which may be serving the ECM/RNDIS interface on the same port.
func (s *apDHCPServer) Start(ctx context.Context) error {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var soErr error
			if err := c.Control(func(fd uintptr) {
				soErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET,
					syscall.SO_BINDTODEVICE, s.iface)
			}); err != nil {
				return err
			}
			return soErr
		},
	}
	pc, err := lc.ListenPacket(ctx, "udp4", "0.0.0.0:67")
	if err != nil {
		return err
	}
	conn := pc.(*net.UDPConn)
	s.conn = conn
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer conn.Close()
		buf := make([]byte, 1500)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("wifi/dhcp: read: %v", err)
				return
			}
			s.handle(conn, buf[:n], addr)
		}
	}()
	// Cancel goroutine when ctx is done.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	return nil
}

// Stop closes the DHCP server.
func (s *apDHCPServer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
	s.wg.Wait()
}

// ClientCount returns the number of active DHCP leases.
func (s *apDHCPServer) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.leases)
}

// handle processes a single DHCP packet.
func (s *apDHCPServer) handle(conn *net.UDPConn, pkt []byte, _ *net.UDPAddr) {
	if len(pkt) < 240 {
		return
	}
	// BOOTP fields
	op := pkt[0]
	if op != 1 { // BOOTREQUEST
		return
	}
	hlen := int(pkt[2])
	if hlen > 16 || hlen < 6 {
		return
	}
	var mac [6]byte
	copy(mac[:], pkt[28:28+hlen])
	xid := pkt[4:8]

	// Parse DHCP message type from options (magic cookie at offset 236)
	if binary.BigEndian.Uint32(pkt[236:240]) != 0x63825363 {
		return
	}
	msgType := dhcpMsgType(pkt[240:])
	if msgType == 0 {
		return
	}

	switch msgType {
	case 1: // DISCOVER
		ip := s.assignIP(mac)
		s.sendReply(conn, pkt, xid, mac, ip, 2 /* OFFER */)
	case 3: // REQUEST
		ip := s.assignIP(mac)
		s.sendReply(conn, pkt, xid, mac, ip, 5 /* ACK */)
	}
}

// assignIP returns the IP assigned to mac, creating a new lease if needed.
func (s *apDHCPServer) assignIP(mac [6]byte) net.IP {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ip, ok := s.leases[mac]; ok {
		return ip
	}
	// Find an unused IP in the pool.
	ip := cloneIP(s.start)
	for !ipAfter(ip, s.end) {
		taken := false
		for _, leased := range s.leases {
			if leased.Equal(ip) {
				taken = true
				break
			}
		}
		if !taken {
			leased := cloneIP(ip)
			s.leases[mac] = leased
			return leased
		}
		ip[3]++
	}
	// Pool exhausted — reuse gateway (should not happen in practice).
	return cloneIP(s.gw)
}

// sendReply sends a DHCP OFFER or ACK.
func (s *apDHCPServer) sendReply(conn *net.UDPConn, req []byte, xid []byte, mac [6]byte, yiaddr net.IP, msgType byte) {
	pkt := make([]byte, 300)
	pkt[0] = 2 // BOOTREPLY
	pkt[1] = req[1]
	pkt[2] = req[2]
	pkt[3] = 0
	copy(pkt[4:8], xid)
	copy(pkt[16:20], yiaddr.To4()) // yiaddr
	copy(pkt[20:24], s.gw.To4())   // siaddr
	copy(pkt[28:28+6], mac[:])     // chaddr

	// Magic cookie
	binary.BigEndian.PutUint32(pkt[236:240], 0x63825363)

	// Options
	opt := pkt[240:]
	i := 0
	// 53: message type
	opt[i] = 53; opt[i+1] = 1; opt[i+2] = msgType; i += 3
	// 1: subnet mask
	opt[i] = 1; opt[i+1] = 4; copy(opt[i+2:], s.mask); i += 6
	// 3: router
	opt[i] = 3; opt[i+1] = 4; copy(opt[i+2:], s.gw.To4()); i += 6
	// 6: DNS
	opt[i] = 6; opt[i+1] = byte(4 * len(s.dns)); i += 2
	for _, d := range s.dns {
		copy(opt[i:], d.To4()); i += 4
	}
	// 51: lease time (1 hour)
	opt[i] = 51; opt[i+1] = 4; binary.BigEndian.PutUint32(opt[i+2:], 3600); i += 6
	// 54: server identifier
	opt[i] = 54; opt[i+1] = 4; copy(opt[i+2:], s.gw.To4()); i += 6
	// 255: end
	opt[i] = 255

	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	_, _ = conn.WriteTo(pkt, dst)
}

// dhcpMsgType extracts the DHCP message type (option 53) from options bytes.
func dhcpMsgType(opts []byte) byte {
	for i := 0; i+2 < len(opts); {
		code := opts[i]
		if code == 255 {
			break
		}
		if code == 0 {
			i++
			continue
		}
		length := int(opts[i+1])
		if i+2+length > len(opts) {
			break
		}
		if code == 53 && length == 1 {
			return opts[i+2]
		}
		i += 2 + length
	}
	return 0
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

// ipAfter reports whether a > b (for IPv4 last-octet comparison).
func ipAfter(a, b net.IP) bool {
	a4, b4 := a.To4(), b.To4()
	if a4 == nil || b4 == nil {
		return false
	}
	return binary.BigEndian.Uint32(a4) > binary.BigEndian.Uint32(b4)
}
