// system/wifi/ap_dhcp.go — minimal DHCP server for the AP subnet
//
// Implements DHCPv4 DISCOVER→OFFER and REQUEST→ACK over raw UDP broadcast.
// Leases are held in memory; clients must re-request after Pi reboot.
// Uses only stdlib — no external dependency required for this subset of DHCP.
//
// The socket is bound to the uap0 interface via SO_BINDTODEVICE so it does not
// conflict with udhcpd which may serve the ECM/RNDIS interface on the same port.
package wifi

import (
	"context"
	"encoding/binary"
	"log"
	"net"
	"sync"
	"syscall"
)

// DHCP message types (RFC 2132 option 53).
const (
	dhcpMsgDiscover byte = 1
	dhcpMsgOffer    byte = 2
	dhcpMsgRequest  byte = 3
	dhcpMsgAck      byte = 5
)

// DHCP option codes (RFC 2132).
const (
	dhcpOptMsgType    byte = 53
	dhcpOptSubnetMask byte = 1
	dhcpOptRouter     byte = 3
	dhcpOptDNS        byte = 6
	dhcpOptLease      byte = 51
	dhcpOptServerID   byte = 54
	dhcpOptEnd        byte = 255
)

const (
	// dhcpMagicCookie is the RFC 2131 magic number at offset 236 of every DHCP packet.
	dhcpMagicCookie uint32 = 0x63825363

	// dhcpLeaseSeconds is the lease duration sent to clients (1 hour).
	dhcpLeaseSeconds uint32 = 3600

	// dhcpPoolStart/End are the last-octet bounds of the address pool (.100–.200).
	// The pool is always relative to the /24 network base (101 addresses max).
	dhcpPoolStart = 100
	dhcpPoolEnd   = 200
)

// apDHCPServer is a minimal DHCPv4 server for the AP subnet.
// It assigns stable leases (same MAC always gets same IP) from a fixed pool.
type apDHCPServer struct {
	iface  string
	gw     net.IP // gateway = uap0 IP
	subnet net.IP // network address
	mask   net.IPMask
	dns    []net.IP
	start  net.IP // first leasable IP (.100)
	end    net.IP // last leasable IP (.200)

	mu     sync.Mutex
	leases map[[6]byte]net.IP // MAC → assigned IP
	taken  map[[4]byte]bool   // IP (4-byte key) → in-use; O(1) pool search

	conn   *net.UDPConn
	wg     sync.WaitGroup
	stopCh chan struct{} // closed by Stop() to unblock the ctx-watcher goroutine
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

	base := netw.IP.To4()
	start := cloneIP(base)
	start[3] = dhcpPoolStart
	end := cloneIP(base)
	end[3] = dhcpPoolEnd

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
		taken:  make(map[[4]byte]bool),
		stopCh: make(chan struct{}),
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
	// Tracked watcher: closes conn when ctx is done OR Stop() is called.
	// Added to wg so Stop() waits for this goroutine to exit.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer conn.Close()
		select {
		case <-ctx.Done():
		case <-s.stopCh:
		}
	}()
	return nil
}

// Stop shuts down the DHCP server and waits for all goroutines to exit,
// including the context-watcher goroutine.
func (s *apDHCPServer) Stop() {
	if s.stopCh != nil {
		close(s.stopCh)
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

	// Verify DHCP magic cookie at offset 236 (RFC 2131).
	if binary.BigEndian.Uint32(pkt[236:240]) != dhcpMagicCookie {
		return
	}
	msgType := dhcpMsgType(pkt[240:])
	if msgType == 0 {
		return
	}

	switch msgType {
	case dhcpMsgDiscover:
		ip := s.assignIP(mac)
		s.sendReply(conn, pkt, xid, mac, ip, dhcpMsgOffer)
	case dhcpMsgRequest:
		ip := s.assignIP(mac)
		s.sendReply(conn, pkt, xid, mac, ip, dhcpMsgAck)
	}
}

// assignIP returns the IP assigned to mac, creating a stable lease if needed.
// Uses an O(1) taken-set so pool searches stay fast even when the pool is full.
// Uses uint32 arithmetic to avoid byte overflow when end[3]==255.
func (s *apDHCPServer) assignIP(mac [6]byte) net.IP {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ip, ok := s.leases[mac]; ok {
		return cloneIP(ip)
	}
	start4 := s.start.To4()
	end4 := s.end.To4()
	if start4 == nil || end4 == nil {
		return cloneIP(s.gw)
	}
	cur := binary.BigEndian.Uint32(start4)
	endVal := binary.BigEndian.Uint32(end4)
	for cur <= endVal {
		var key [4]byte
		binary.BigEndian.PutUint32(key[:], cur)
		if !s.taken[key] {
			leased := make(net.IP, 4)
			binary.BigEndian.PutUint32(leased, cur)
			s.leases[mac] = leased
			s.taken[key] = true
			return leased
		}
		cur++
	}
	log.Printf("wifi/dhcp: address pool exhausted (%d leases)", len(s.leases))
	return cloneIP(s.gw) // fallback: should not happen with <101 clients
}

// sendReply broadcasts a DHCP OFFER or ACK to port 68.
// pkt layout follows RFC 2131; options follow RFC 2132.
func (s *apDHCPServer) sendReply(conn *net.UDPConn, req []byte, xid []byte, mac [6]byte, yiaddr net.IP, msgType byte) {
	pkt := make([]byte, 300)
	pkt[0] = 2 // op: BOOTREPLY
	pkt[1] = req[1]
	pkt[2] = req[2]
	pkt[3] = 0
	copy(pkt[4:8], xid)
	copy(pkt[16:20], yiaddr.To4()) // yiaddr: offered address
	copy(pkt[20:24], s.gw.To4())   // siaddr: server address
	copy(pkt[28:28+6], mac[:])     // chaddr: client hardware address

	binary.BigEndian.PutUint32(pkt[236:240], dhcpMagicCookie)

	opt := pkt[240:]
	i := 0
	opt[i] = dhcpOptMsgType; opt[i+1] = 1; opt[i+2] = msgType; i += 3
	opt[i] = dhcpOptSubnetMask; opt[i+1] = 4; copy(opt[i+2:], s.mask); i += 6
	opt[i] = dhcpOptRouter; opt[i+1] = 4; copy(opt[i+2:], s.gw.To4()); i += 6
	opt[i] = dhcpOptDNS; opt[i+1] = byte(4 * len(s.dns)); i += 2
	for _, d := range s.dns {
		copy(opt[i:], d.To4()); i += 4
	}
	opt[i] = dhcpOptLease; opt[i+1] = 4; binary.BigEndian.PutUint32(opt[i+2:], dhcpLeaseSeconds); i += 6
	opt[i] = dhcpOptServerID; opt[i+1] = 4; copy(opt[i+2:], s.gw.To4()); i += 6
	opt[i] = dhcpOptEnd

	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	if _, err := conn.WriteTo(pkt, dst); err != nil {
		log.Printf("wifi/dhcp: send %s to %s: %v",
			map[byte]string{dhcpMsgOffer: "OFFER", dhcpMsgAck: "ACK"}[msgType],
			yiaddr, err)
	}
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
