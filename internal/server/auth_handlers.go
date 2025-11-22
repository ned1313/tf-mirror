package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo represents basic user information
type UserInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// handleLogin authenticates a user and returns a JWT token
// POST /admin/api/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate inputs
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "missing_credentials", "Username and password are required")
		return
	}

	// Get user by username
	userRepo := database.NewUserRepository(s.db)
	user, err := userRepo.GetByUsername(r.Context(), req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to authenticate")
		return
	}

	if user == nil || !user.Active {
		respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	// Verify password
	if err := s.authService.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	// Generate JWT token
	token, jti, expiresAt, err := s.authService.GenerateToken(user.ID, user.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "token_error", "Failed to generate token")
		return
	}

	// Create session record
	sessionRepo := database.NewSessionRepository(s.db)
	session := &database.AdminSession{
		UserID:    user.ID,
		TokenJTI:  jti,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	// Get IP and User-Agent if available
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		session.IPAddress = sql.NullString{String: ip, Valid: true}
	} else if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		session.IPAddress = sql.NullString{String: strings.Split(ip, ",")[0], Valid: true}
	} else {
		session.IPAddress = sql.NullString{String: r.RemoteAddr, Valid: true}
	}

	if ua := r.Header.Get("User-Agent"); ua != "" {
		session.UserAgent = sql.NullString{String: ua, Valid: true}
	}

	if err := sessionRepo.Create(r.Context(), session); err != nil {
		respondError(w, http.StatusInternalServerError, "session_error", "Failed to create session")
		return
	}

	// Update last login timestamp
	if err := userRepo.UpdateLastLogin(r.Context(), user.ID); err != nil {
		// Log but don't fail
		s.logger.Printf("Failed to update last login for user %d: %v", user.ID, err)
	}

	// Return token
	response := LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleLogout revokes the current session
// POST /admin/api/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		respondError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate token to get JTI
	claims, err := s.authService.ValidateToken(tokenString)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
		return
	}

	// Revoke session
	sessionRepo := database.NewSessionRepository(s.db)
	if err := sessionRepo.RevokeByTokenJTI(r.Context(), claims.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "revoke_error", "Failed to revoke session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// authMiddleware validates JWT tokens and injects user context
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(w, http.StatusUnauthorized, "missing_token", "Authorization token required")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate token
		claims, err := s.authService.ValidateToken(tokenString)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}

		// Check if session is revoked
		sessionRepo := database.NewSessionRepository(s.db)
		session, err := sessionRepo.GetByTokenJTI(r.Context(), claims.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "session_error", "Failed to validate session")
			return
		}

		if session == nil || session.Revoked {
			respondError(w, http.StatusUnauthorized, "session_revoked", "Session has been revoked")
			return
		}

		// Check if session has expired
		if time.Now().After(session.ExpiresAt) {
			respondError(w, http.StatusUnauthorized, "session_expired", "Session has expired")
			return
		}

		// TODO: Add user ID and username to request context
		// For now, just pass through

		next.ServeHTTP(w, r)
	})
}
