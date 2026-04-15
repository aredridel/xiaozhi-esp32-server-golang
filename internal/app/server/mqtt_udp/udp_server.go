package mqtt_udp

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	. "xiaozhi-esp32-server-golang/logger"
)

// UDPServer UDP server structure
/*
type UDPServer struct {
	conn       *net.UDPConn
	sessions   map[string]*Session
	mqttServer *MqttServer
	udpPort    int
	sync.RWMutex
}*/

type UdpServer struct {
	conn          *net.UDPConn
	udpPort       int      // udp server listen port
	externalHost  string   // udp server external host
	externalPort  int      // udp server external port
	nonce2Session sync.Map //nonce => UdpSession
	addr2Session  sync.Map //addr => UdpSession
	mqttAdapter   *MqttUdpAdapter
	sync.RWMutex
}

// NewUDPServer Create new UDP server
func NewUDPServer(udpPort int, externalHost string, externalPort int) *UdpServer {
	return &UdpServer{
		udpPort:       udpPort,
		externalHost:  externalHost,
		externalPort:  externalPort,
		nonce2Session: sync.Map{},
		addr2Session:  sync.Map{},
	}
}

// Start Start UDP server
func (s *UdpServer) Start() error {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: s.udpPort,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen on UDP: %v", err)
	}

	s.conn = conn
	Infof("UDP server started on %s:%d", "0.0.0.0", s.udpPort)

	// Start session cleanup
	//go s.cleanupSessions()

	// Start packet processing
	go s.handlePackets()

	return nil
}

// Close Close UDP server, causing handlePackets to exit
func (s *UdpServer) Close() error {
	s.Lock()
	conn := s.conn
	s.conn = nil
	s.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// handlePackets Process received packets
func (s *UdpServer) handlePackets() {
	buffer := make([]byte, 4096) // usedefaultofbuffer区size
	for {
		s.RLock()
		conn := s.conn
		s.RUnlock()
		if conn == nil {
			return
		}
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			s.RLock()
			closed := s.conn == nil
			s.RUnlock()
			if closed {
				return
			}
			Errorf("Failed to read UDP data: %v", err)
			continue
		}

		// Copy data to avoid concurrent modification
		data := make([]byte, n)
		copy(data, buffer[:n])

		// Process packet
		s.processPacket(addr, data)
	}
}

func (s *UdpServer) getSessionByNonce(connID string) *UdpSession {
	val, ok := s.nonce2Session.Load(connID)
	if ok {
		return val.(*UdpSession)
	}
	return nil
}

// processPacket Process single packet
func (s *UdpServer) processPacket(addr *net.UDPAddr, data []byte) {
	// Check packet size
	if len(data) < 16 {
		Warn("Packet too small")
		return
	}

	fullNonce := data[:16]
	connID := fullNonce[4:8] // 取5-8byteasisjoinid
	strConnID := hex.EncodeToString(connID)
	udpSession := s.getSessionByNonce(strConnID)
	if udpSession == nil {
		Warnf("session does not exist addr: %s, connID: %s", addr, strConnID)
		return
	}
	addrSession := s.getUdpSession(addr)
	if addrSession != udpSession {
		if addrSession != nil {
			s.removeUdpSession(addr)
		}
		s.rebindSessionAddr(addr, udpSession)
	}

	if udpSession == nil {
		Warnf("udpSession does not exist addr: %s", addr)
		return
	}

	// Update last active time
	udpSession.LastActive = time.Now()

	decrypted, err := udpSession.Decrypt(data)
	if err != nil {
		Errorf("addr: %s decryption failed: %v", addr, err)
		return
	}
	Debugf("Received audio data, addr: %s, size: %d bytes", addr, len(decrypted))
	ok, err := udpSession.RecvData(decrypted)
	if err != nil {
		Errorf("addr: %s failed to receive data: %v", addr, err)
		return
	}
	if !ok {
		Warnf("addr: %s failed to receive data, channel is full", addr)
		return
	}
	/*select {
	case udpSession.RecvChannel <- decrypted:
		return
	default:
		Warnf("udpSession.RecvChannel is full, addr: %s", addr)
	}*/
}

// cleanupSessions Clean up expired sessions
func (s *UdpServer) cleanupSessions() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		now := time.Now()
		s.nonce2Session.Range(func(key, value interface{}) bool {
			session := value.(*UdpSession)
			if now.Sub(session.LastActive) > 5*time.Minute {
				s.nonce2Session.Delete(key)
				Infof("Cleaned up expired session: %s", key)
			}
			return true
		})
	}
}

// CreateSession Create new session
func (s *UdpServer) CreateSession(deviceId, clientId string) *UdpSession {
	// Generate session ID
	sessionID := generateSessionID()

	// Generate AES key
	key := make([]byte, 16)
	rand.Read(key)

	// Generate 4-byte connection id
	connID := make([]byte, 4)
	rand.Read(connID)
	strConnID := hex.EncodeToString(connID)

	// 4-byte timestamp
	timestamp := make([]byte, 4)
	binary.BigEndian.PutUint32(timestamp, uint32(time.Now().Unix()))

	// Concatenate nonce: 4-byte connection id + 4-byte timestamp
	nonce := append(connID, timestamp...)

	// Create AES block
	block, err := aes.NewCipher(key)
	if err != nil {
		Errorf("Failed to create AES block: %v", err)
		return nil
	}

	// Convert key to [16]byte
	aesKey := [16]byte{}
	copy(aesKey[:], key)

	// Convert nonce to [8]byte
	nonceBytes := [8]byte{}
	copy(nonceBytes[:], nonce)

	// Create session
	session := &UdpSession{
		ID:          sessionID,
		ConnId:      strConnID,
		ClientId:    clientId,
		DeviceId:    deviceId,
		AesKey:      aesKey,
		Nonce:       nonceBytes, // Save original nonce template
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		Block:       block,
		RecvChannel: make(chan []byte, 100),
		SendChannel: make(chan []byte, 100),
		Status:      UdpSessionStatusActive,
		Lock:        sync.Mutex{},
	}
	// Send audio data through channel, stop when channel is closed
	go func() {
		for data := range session.SendChannel {
			remoteAddr := session.WaitRemoteAddr(2 * time.Second)
			if remoteAddr == nil {
				dropped := 1 + session.DrainPendingAudio()
				Warnf("UDP remote address not established, TTS audio dropped: device=%s, connId=%s, dropped=%d", session.DeviceId, session.ConnId, dropped)
				continue
			}
			encrypted, err := session.Encrypt(data)
			if err != nil {
				Errorf("Encryption failed: %v", err)
				continue
			}
			//Debugf("Sending audio data, nonce: %s, size: %d bytes", hex.EncodeToString(encrypted[:16]), len(encrypted))
			_, err = s.conn.WriteToUDP(encrypted, remoteAddr)
			if err != nil {
				Errorf("Failed to send audio data: %v", err)
				continue
			}
			//Debugf("Audio data sent successfully, nonce: %s, size: %d bytes, bytes sent: %d", hex.EncodeToString(encrypted[:16]), len(encrypted), n)
		}
	}()

	// Use only connection id (first 4 bytes) as key
	s.SetNonce2Session(strConnID, session)

	return session
}

// CloseSession Close session
func (s *UdpServer) CloseSession(connID string) {
	session := s.getSessionByNonce(connID)
	s.CloseSessionByRef(session)
}

// ClearSessionAddrBinding Clear UDP address binding for session corresponding to connID, do not destroy the session itself
func (s *UdpServer) ClearSessionAddrBinding(connID string) {
	session := s.getSessionByNonce(connID)
	if session == nil {
		return
	}
	s.clearSessionAddrBindings(session)
}

func (s *UdpServer) SetNonce2Session(connID string, session *UdpSession) {
	Debugf("SetNonce2Session, connID: %s, session: %+v", connID, session)
	s.nonce2Session.Store(connID, session)
}

// GetSession Get session information
func (s *UdpServer) GetNonce(connID string) *UdpSession {
	val, ok := s.nonce2Session.Load(connID)
	if ok {
		return val.(*UdpSession)
	}
	return nil
}

// generateSessionID Generate session ID
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *UdpServer) getUdpSession(addr *net.UDPAddr) *UdpSession {
	val, ok := s.addr2Session.Load(addr.String())
	if ok {
		return val.(*UdpSession)
	}
	return nil
}

func (s *UdpServer) addUdpSession(addr *net.UDPAddr, session *UdpSession) {
	s.addr2Session.Store(addr.String(), session)
}

func (s *UdpServer) removeUdpSession(addr *net.UDPAddr) {
	s.addr2Session.Delete(addr.String())
}

func (s *UdpServer) CloseSessionByRef(session *UdpSession) {
	if session == nil {
		return
	}
	s.clearSessionAddrBindings(session)
	s.nonce2Session.Delete(session.ConnId)
	session.Destroy()
}

func (s *UdpServer) clearSessionAddrBindings(session *UdpSession) {
	if session == nil {
		return
	}
	s.addr2Session.Range(func(key, value interface{}) bool {
		if value == session {
			s.addr2Session.Delete(key)
		}
		return true
	})
	session.SetRemoteAddr(nil)
}

func (s *UdpServer) rebindSessionAddr(addr *net.UDPAddr, session *UdpSession) {
	if addr == nil || session == nil {
		return
	}
	s.clearSessionAddrBindings(session)
	session.SetRemoteAddr(addr)
	s.addUdpSession(addr, session)
}
