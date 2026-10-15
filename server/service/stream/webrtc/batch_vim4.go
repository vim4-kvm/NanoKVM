//go:build linux && vim4

package webrtc

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"unsafe"

	"github.com/pion/transport/v3"
	"github.com/pion/transport/v3/stdnet"
	"golang.org/x/sys/unix"
)

// Keep bursts below Linux's 64-segment limit. Large offloaded bursts caused
// receiver loss at 4K60/60 Mbps on the wired LAN; eight segments retained
// batching without that loss in H.264/H.265 motion tests.
const (
	maxGSOSegments = 8
	maxGSOBytes    = 65000
	maxDatagram    = 1500
)

// udpBatcher collects the UDP datagrams pion writes for one video frame and
// sends short equal-sized runs with UDP_SEGMENT to amortize send syscalls
// without releasing an entire frame as one large offloaded burst.
type udpBatcher struct {
	*stdnet.Net
	mu      sync.Mutex
	active  bool
	pending []*batchConn
}

func newUDPBatcher() (*udpBatcher, error) {
	n, err := stdnet.NewNet()
	if err != nil {
		return nil, err
	}
	return &udpBatcher{Net: n}, nil
}

func (b *udpBatcher) ListenUDP(network string, addr *net.UDPAddr) (transport.UDPConn, error) {
	conn, err := b.Net.ListenUDP(network, addr)
	if err != nil {
		return nil, err
	}
	// mDNS needs the concrete socket for multicast control messages.
	udp, ok := conn.(*net.UDPConn)
	if !ok || addr != nil && addr.IP.IsMulticast() {
		return conn, nil
	}
	raw, err := udp.SyscallConn()
	if err != nil {
		return conn, nil
	}
	// Bursts are larger than the default send buffer; avoid blocking mid-frame.
	_ = udp.SetWriteBuffer(4 << 20)
	return &batchConn{UDPConn: udp, raw: raw, batcher: b}, nil
}

// begin starts collecting writes; flush must follow.
func (b *udpBatcher) begin() {
	b.mu.Lock()
	b.active = true
	b.mu.Unlock()
}

// flush sends collected datagrams and stops collecting.
func (b *udpBatcher) flush() error {
	b.mu.Lock()
	b.active = false
	pending := b.pending
	b.pending = nil
	b.mu.Unlock()
	var errs []error
	for _, c := range pending {
		errs = append(errs, c.flush())
	}
	return errors.Join(errs...)
}

type batchConn struct {
	*net.UDPConn
	raw interface {
		Write(func(uintptr) bool) error
	}
	batcher *udpBatcher

	mu    sync.Mutex
	addr  *net.UDPAddr
	buf   []byte
	sizes []int
	noGSO bool
}

func (c *batchConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	b := c.batcher
	b.mu.Lock()
	if !ok || !b.active {
		b.mu.Unlock()
		return c.UDPConn.WriteTo(p, addr)
	}
	c.mu.Lock()
	if len(c.sizes) == 0 {
		b.pending = append(b.pending, c)
	}
	b.mu.Unlock()
	defer c.mu.Unlock()
	if len(c.sizes) > 0 && (c.addr.Port != udpAddr.Port || !c.addr.IP.Equal(udpAddr.IP)) {
		if err := c.sendLocked(); err != nil {
			return 0, err
		}
	}
	c.addr = udpAddr
	c.buf = append(c.buf, p...)
	c.sizes = append(c.sizes, len(p))
	// Send each complete GSO run as soon as it is full so transmission
	// overlaps with packetizing and encrypting the rest of the frame.
	if len(c.sizes) >= maxGSOSegments || len(c.buf)+maxDatagram > maxGSOBytes {
		if err := c.sendLocked(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (c *batchConn) flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLocked()
}

// sendLocked sends runs where every segment but the last has the same size.
func (c *batchConn) sendLocked() error {
	defer func() { c.buf, c.sizes = c.buf[:0], c.sizes[:0] }()
	sa := &unix.SockaddrInet4{Port: c.addr.Port}
	ip4 := c.addr.IP.To4()
	if ip4 == nil {
		// IPv6 candidates keep the per-packet path.
		offset := 0
		for _, size := range c.sizes {
			if _, err := c.UDPConn.WriteTo(c.buf[offset:offset+size], c.addr); err != nil {
				return err
			}
			offset += size
		}
		return nil
	}
	copy(sa.Addr[:], ip4)
	offset := 0
	for start := 0; start < len(c.sizes); {
		end, segment := start+1, c.sizes[start]
		for end < len(c.sizes) && c.sizes[end-1] == segment && c.sizes[end] <= segment {
			end++
		}
		length := 0
		for _, size := range c.sizes[start:end] {
			length += size
		}
		if err := c.sendRun(c.buf[offset:offset+length], c.sizes[start:end], segment, sa); err != nil {
			return err
		}
		offset += length
		start = end
	}
	return nil
}

func (c *batchConn) sendRun(data []byte, sizes []int, segment int, sa unix.Sockaddr) error {
	if len(sizes) > 1 && !c.noGSO {
		err := c.send(data, segment, true, sa)
		if !errors.Is(err, unix.EIO) && !errors.Is(err, unix.EINVAL) && !errors.Is(err, unix.ENOPROTOOPT) {
			return err
		}
		// The kernel or device refused segmentation; keep the per-packet path.
		c.noGSO = true
	}
	offset := 0
	for _, size := range sizes {
		if err := c.send(data[offset:offset+size], 0, false, sa); err != nil {
			return err
		}
		offset += size
	}
	return nil
}

func (c *batchConn) send(data []byte, segment int, gso bool, sa unix.Sockaddr) error {
	var oob []byte
	if gso {
		oob = make([]byte, unix.CmsgSpace(2))
		h := (*unix.Cmsghdr)(unsafe.Pointer(&oob[0]))
		h.Level, h.Type = unix.SOL_UDP, unix.UDP_SEGMENT
		h.SetLen(unix.CmsgLen(2))
		binary.NativeEndian.PutUint16(oob[unix.CmsgLen(0):], uint16(segment))
	}
	var sendErr error
	err := c.raw.Write(func(fd uintptr) bool {
		sendErr = unix.Sendmsg(int(fd), data, oob, sa, 0)
		return sendErr != unix.EAGAIN
	})
	if err != nil {
		return err
	}
	return sendErr
}
