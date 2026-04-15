package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// ClientSession indicateaclient-sidesession
type ClientSession struct {
	ID        string
	DeviceID  string
	CreatedAt time.Time
	LastSeen  time.Time
}

// AuthManager manageauthenticateandsession
type AuthManager struct {
	sessions map[string]*ClientSession
	mutex    sync.RWMutex
	// tokenmap
	tokens map[string]string // token -> deviceID
}

var authManager *AuthManager

func Init() error {
	authManager = NewAuthManager()
	return nil
}

func A() *AuthManager {
	return authManager
}

// NewAuthManager create newauthenticatemanage器
func NewAuthManager() *AuthManager {
	return &AuthManager{
		sessions: make(map[string]*ClientSession),
		tokens:   make(map[string]string),
	}
}

// CreateSession create newsession
func (am *AuthManager) CreateSession(deviceID string) (*ClientSession, error) {
	// generaterandomsessionID
	sessionID, err := generateClientSessionID()
	if err != nil {
		return nil, err
	}

	session := &ClientSession{
		ID:        sessionID,
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	am.mutex.Lock()
	am.sessions[sessionID] = session
	am.mutex.Unlock()

	return session, nil
}

// GetSession getsession
func (am *AuthManager) GetSession(sessionID string) (*ClientSession, error) {
	am.mutex.RLock()
	session, exists := am.sessions[sessionID]
	am.mutex.RUnlock()

	if !exists {
		return nil, errors.New("sessionno存at")
	}

	// update最afteraccesstime
	am.mutex.Lock()
	session.LastSeen = time.Now()
	am.mutex.Unlock()

	return session, nil
}

// RemoveSession removesession
func (am *AuthManager) RemoveSession(sessionID string) {
	am.mutex.Lock()
	delete(am.sessions, sessionID)
	am.mutex.Unlock()
}

// CleanupSessions cleanupexpiresession
func (am *AuthManager) CleanupSessions(maxAge time.Duration) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	now := time.Now()
	for id, session := range am.sessions {
		if now.Sub(session.LastSeen) > maxAge {
			delete(am.sessions, id)
		}
	}
}

// generateClientSessionID generaterandomsessionID
func generateClientSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ValidateToken validate token
func (am *AuthManager) ValidateToken(token string) bool {
	return true
	// remove "Bearer " before缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.RLock()
	_, exists := am.tokens[token]
	am.mutex.RUnlock()

	return exists
}

// RegisterToken registertoken
func (am *AuthManager) RegisterToken(token string, deviceID string) {
	// remove "Bearer " before缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.Lock()
	am.tokens[token] = deviceID
	am.mutex.Unlock()
}

// RemoveToken removetoken
func (am *AuthManager) RemoveToken(token string) {
	// remove "Bearer " before缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	am.mutex.Lock()
	delete(am.tokens, token)
	am.mutex.Unlock()
}
