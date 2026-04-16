package mqtt_udp

import (
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	UdpSessionStatusActive = "active"
	UdpSessionStatusClosed = "closed"
)

// Session indicates a UDP session
type UdpSession struct {
	ID          string
	Conn        *net.UDPConn // udp conn
	ConnId      string
	ClientId    string
	DeviceId    string
	AesKey      [16]byte // random 16 bytes
	Nonce       [8]byte  // store original nonce template 8 bytes
	CreatedAt   time.Time
	LastActive  time.Time
	RemoteAddr  *net.UDPAddr // remote addr
	LocalSeq    uint32
	Block       cipher.Block
	RemoteSeq   uint32
	RecvChannel chan []byte // receive audio data
	SendChannel chan []byte // send audio data
	Status      string
	Lock        sync.Mutex
}

func (s *UdpSession) SetRemoteAddr(addr *net.UDPAddr) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	s.RemoteAddr = addr
}

func (s *UdpSession) GetRemoteAddr() *net.UDPAddr {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.RemoteAddr == nil {
		return nil
	}
	addrCopy := *s.RemoteAddr
	return &addrCopy
}

func (s *UdpSession) IsClosed() bool {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	return s.Status == UdpSessionStatusClosed
}

func (s *UdpSession) WaitRemoteAddr(timeout time.Duration) *net.UDPAddr {
	if timeout <= 0 {
		return s.GetRemoteAddr()
	}

	deadline := time.Now().Add(timeout)
	for {
		if addr := s.GetRemoteAddr(); addr != nil {
			return addr
		}
		if s.IsClosed() || !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *UdpSession) DrainPendingAudio() int {
	drained := 0
	for {
		select {
		case <-s.SendChannel:
			drained++
		default:
			return drained
		}
	}
}

// decrypt decrypt data
func (s *UdpSession) Decrypt(data []byte) ([]byte, error) {
	// detach nonce and ciphertext
	nonce := data[:16] // use 16 byte nonce
	ciphertext := data[16:]

	// extract sequence number
	seqNum := binary.BigEndian.Uint32(data[12:16])

	// inspect sequence number
	/*if seqNum < s.RemoteSeq {
		return nil, fmt.Errorf("sequence number expired: got %d, expected >= %d", seqNum, s.RemoteSeq)
	}*/
	s.RemoteSeq = seqNum

	// decrypt data
	stream := cipher.NewCTR(s.Block, nonce)
	decrypted := make([]byte, len(ciphertext))
	stream.XORKeyStream(decrypted, ciphertext)

	return decrypted, nil
}

// encrypt encrypt data
func (s *UdpSession) Encrypt(data []byte) ([]byte, error) {
	// pre-allocate memory, avoid expansion
	encrypted := make([]byte, 16+len(data))

	// build nonce (16 byte)
	encrypted[0] = 0x01                                          // package type
	binary.BigEndian.PutUint16(encrypted[2:], uint16(len(data))) // data length
	copy(encrypted[4:12], s.Nonce[:])                            // 8 byte nonce
	s.LocalSeq++
	binary.BigEndian.PutUint32(encrypted[12:], s.LocalSeq) // sequence number

	// encrypt data
	stream := cipher.NewCTR(s.Block, encrypted[:16]) // use 16 byte as IV
	stream.XORKeyStream(encrypted[16:], data)

	return encrypted, nil
}

func (s *UdpSession) GetAesKeyAndNonce() (string, string) {
	// process
	strAesKey := hex.EncodeToString(s.AesKey[:])

	// construct fullNonce: prefix 2 bytes 0100 + length 2 bytes 0000 + real nonce (8 bytes) + seq (4 bytes 00000000)
	prefix := []byte{0x01, 0x00}
	length := []byte{0x00, 0x00}
	seq := []byte{0x00, 0x00, 0x00, 0x00}
	fullNonce := append(append(append(prefix, length...), s.Nonce[:]...), seq...)
	strFullNonce := hex.EncodeToString(fullNonce)

	return strAesKey, strFullNonce
}

func (s *UdpSession) RecvData(data []byte) (bool, error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return false, nil
	}
	select {
	case s.RecvChannel <- data:
		return true, nil
	default:
		return false, fmt.Errorf("recv channel is full")
	}
}

// SendAudioData send audio data
func (s *UdpSession) SendAudioData(data []byte) (bool, error) {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return false, nil
	}
	select {
	case s.SendChannel <- data:
		return true, nil
	default:
		return false, fmt.Errorf("send channel is full")
	}
}

func (s *UdpSession) Destroy() {
	s.Lock.Lock()
	defer s.Lock.Unlock()
	if s.Status == UdpSessionStatusClosed {
		return
	}
	s.Status = UdpSessionStatusClosed
	close(s.RecvChannel)
	close(s.SendChannel)
}
