package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/dumacp/demowalletinji/internal/models"
)

// AuthService handles authentication, sessions and challenges
type AuthService struct {
	sessions   map[string]*models.Session
	challenges map[string]*models.Challenge
	mutex      sync.RWMutex
	ttl        time.Duration
}

// NewAuthService creates a new authentication service
func NewAuthService(ttl time.Duration) *AuthService {
	service := &AuthService{
		sessions:   make(map[string]*models.Session),
		challenges: make(map[string]*models.Challenge),
		ttl:        ttl,
	}

	// Start cleanup routine
	go service.cleanup()

	return service
}

// CreateSession creates a new session with unique ID
func (s *AuthService) CreateSession(sessionType string) (*models.Session, error) {
	sessionID, err := s.generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	session := &models.Session{
		ID:        sessionID,
		Type:      sessionType,
		Status:    "created",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.ttl),
	}

	s.mutex.Lock()
	s.sessions[sessionID] = session
	s.mutex.Unlock()

	return session, nil
}

// CreateVerificationSession creates a session with a specific ID (for verification states)
func (s *AuthService) CreateVerificationSession(sessionID, sessionType string) error {
	session := &models.Session{
		ID:        sessionID,
		Type:      sessionType,
		Status:    "pending_verification",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.ttl),
	}

	s.mutex.Lock()
	s.sessions[sessionID] = session
	s.mutex.Unlock()

	return nil
}

// GetSession retrieves a session by ID
func (s *AuthService) GetSession(sessionID string) (*models.Session, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// UpdateSession updates an entire session object
func (s *AuthService) UpdateSession(sessionID string, session *models.Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	s.sessions[sessionID] = session
	return nil
}

// UpdateSessionStatus updates the status of an existing session
func (s *AuthService) UpdateSessionStatus(sessionID, status string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Status = status
	session.UpdatedAt = time.Now()

	return nil
}

// DeleteSession removes a session
func (s *AuthService) DeleteSession(sessionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.sessions, sessionID)
}

// CreateChallenge creates a new verification challenge
func (s *AuthService) CreateChallenge(sessionID, vpPolicyID string) (*models.Challenge, error) {
	challengeID, err := s.generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge ID: %w", err)
	}

	challenge := &models.Challenge{
		ID:         challengeID,
		SessionID:  sessionID,
		VpPolicyID: vpPolicyID,
		Status:     "pending",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(s.ttl),
	}

	s.mutex.Lock()
	s.challenges[challengeID] = challenge
	s.mutex.Unlock()

	return challenge, nil
}

// GetChallenge retrieves a challenge by ID
func (s *AuthService) GetChallenge(challengeID string) (*models.Challenge, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	challenge, exists := s.challenges[challengeID]
	if !exists {
		return nil, false
	}

	// Check if challenge is expired
	if time.Now().After(challenge.ExpiresAt) {
		return nil, false
	}

	return challenge, true
}

// UpdateChallengeStatus updates the status of an existing challenge
func (s *AuthService) UpdateChallengeStatus(challengeID, status string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	challenge, exists := s.challenges[challengeID]
	if !exists {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}

	challenge.Status = status
	challenge.UpdatedAt = time.Now()

	return nil
}

// GetSessionChallenges returns all challenges for a session
func (s *AuthService) GetSessionChallenges(sessionID string) []*models.Challenge {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var challenges []*models.Challenge
	for _, challenge := range s.challenges {
		if challenge.SessionID == sessionID && time.Now().Before(challenge.ExpiresAt) {
			challenges = append(challenges, challenge)
		}
	}

	return challenges
}

// generateID generates a cryptographically random ID
func (s *AuthService) generateID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// cleanup periodically removes expired sessions and challenges
func (s *AuthService) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		s.mutex.Lock()

		// Clean up expired sessions
		for id, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, id)
			}
		}

		// Clean up expired challenges
		for id, challenge := range s.challenges {
			if now.After(challenge.ExpiresAt) {
				delete(s.challenges, id)
			}
		}

		s.mutex.Unlock()
	}
}
