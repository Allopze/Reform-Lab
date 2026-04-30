package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/api/middleware"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const sessionCookieName = "reform_session"
const sessionCookieMaxAgeSeconds = 72 * 60 * 60

// AuthHandler handles POST /api/auth/register and /api/auth/login.
type AuthHandler struct {
	Auth               *auth.Service
	Email              *email.Service
	Queue              queue.JobQueue
	PasswordResets     repository.PasswordResetRepository
	EmailVerifications repository.EmailVerificationRepository
	Users              repository.UserRepository
	AppURL             string
	Audit              repository.AuditRepository
	Logger             zerolog.Logger
	TrustProxyHeaders  bool
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Team     string `json:"team"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type emailVerificationConfirmRequest struct {
	Token string `json:"token"`
}

// Register handles POST /api/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Team = strings.TrimSpace(req.Team)

	if req.Name == "" || req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	result, err := h.Auth.Register(r.Context(), auth.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Team:     req.Team,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			respondError(w, http.StatusConflict, "email already registered")
			return
		}
		if errors.Is(err, domain.ErrBootstrapAdminRequired) {
			respondError(w, http.StatusConflict, "initial admin must be bootstrapped explicitly before public registration")
			return
		}
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	h.writeSessionCookie(w, r, result.SessionToken)

	// Enqueue welcome email (best-effort, never blocks registration).
	if h.Email != nil && h.Queue != nil && h.Email.Configured(r.Context()) {
		err := h.Queue.EnqueueEmail(r.Context(), queue.EmailTaskPayload{
			TemplateKey: "welcome",
			To:          result.User.Email,
			Vars: map[string]string{
				"Name":    result.User.Name,
				"Email":   result.User.Email,
				"AppName": "Reform Lab",
				"Year":    fmt.Sprintf("%d", time.Now().Year()),
			},
		}, queue.TaskOptions{MaxRetries: 3, Timeout: 30 * time.Second})
		if err != nil {
			h.Logger.Warn().Err(err).Str("user_id", result.User.ID.String()).Msg("failed to enqueue welcome email")
		}
	}
	if err := h.enqueueEmailVerification(r, result.User); err != nil {
		h.Logger.Warn().Err(err).Str("user_id", result.User.ID.String()).Msg("failed to enqueue email verification")
	}

	respondJSON(w, http.StatusCreated, result)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			h.auditSession(r, domain.AuditSessionLoginFailed, nil, map[string]interface{}{
				"email":  req.Email,
				"reason": "invalid_credentials",
			})
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		if errors.Is(err, domain.ErrUserSuspended) {
			h.auditSession(r, domain.AuditSessionLoginFailed, nil, map[string]interface{}{
				"email":  req.Email,
				"reason": "user_suspended",
			})
			respondError(w, http.StatusForbidden, "user suspended")
			return
		}
		h.auditSession(r, domain.AuditSessionLoginFailed, nil, map[string]interface{}{
			"email":  req.Email,
			"reason": "internal_error",
		})
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	h.writeSessionCookie(w, r, result.SessionToken)
	h.auditSession(r, domain.AuditSessionLogin, &result.User.ID, map[string]interface{}{
		"role": result.User.Role,
	})
	respondJSON(w, http.StatusOK, result)
}

// Logout clears the current session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	details := map[string]interface{}{}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" && h.Auth != nil {
		if claims, validateErr := h.Auth.ValidateToken(cookie.Value); validateErr == nil {
			if parsedID, parseErr := uuid.Parse(claims.Subject); parseErr == nil {
				userID = &parsedID
				details["role"] = claims.Role
			}
		}
	}
	h.auditSession(r, domain.AuditSessionLogout, userID, details)
	h.clearSessionCookie(w, r)
	respondJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// Me handles GET /api/auth/me — returns current user from context.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	respondJSON(w, http.StatusOK, u)
}

// PasswordResetRequest handles POST /api/auth/password-reset/request.
// It creates a one-time reset token and sends it via email.
func (h *AuthHandler) PasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil || h.Users == nil || h.PasswordResets == nil {
		respondError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}
	if h.Email == nil || h.Queue == nil || !h.Email.Configured(r.Context()) {
		respondError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}
	if strings.TrimSpace(h.AppURL) == "" {
		respondError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}

	var req passwordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}

	u, err := h.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			h.auditSession(r, domain.AuditPasswordResetRequested, nil, map[string]interface{}{"email": req.Email, "result": "no_user"})
			respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		respondError(w, http.StatusInternalServerError, "request failed")
		return
	}
	if u == nil {
		// Avoid leaking whether an email exists.
		h.auditSession(r, domain.AuditPasswordResetRequested, nil, map[string]interface{}{"email": req.Email, "result": "no_user"})
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	rawToken, err := newResetToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "request failed")
		return
	}
	tokenHash := hashResetToken(rawToken)
	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Hour)

	// Keep it simple: single active token per user.
	_ = h.PasswordResets.DeleteForUser(r.Context(), u.ID)
	if err := h.PasswordResets.Create(r.Context(), u.ID, tokenHash, expiresAt, now); err != nil {
		respondError(w, http.StatusInternalServerError, "request failed")
		return
	}

	resetURL := buildPasswordResetURL(h.AppURL, rawToken)
	err = h.Queue.EnqueueEmail(r.Context(), queue.EmailTaskPayload{
		TemplateKey: "password-reset",
		To:          u.Email,
		Vars: map[string]string{
			"Name":     u.Name,
			"AppName":  "Reform Lab",
			"ResetURL": resetURL,
			"Year":     fmt.Sprintf("%d", time.Now().Year()),
		},
	}, queue.TaskOptions{MaxRetries: 3, Timeout: 30 * time.Second})
	if err != nil {
		h.Logger.Warn().Err(err).Str("user_id", u.ID.String()).Msg("failed to enqueue password reset email")
		respondError(w, http.StatusInternalServerError, "request failed")
		return
	}

	h.auditSession(r, domain.AuditPasswordResetRequested, &u.ID, map[string]interface{}{"email": u.Email, "result": "enqueued"})
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PasswordResetConfirm handles POST /api/auth/password-reset/confirm.
// It consumes a token, updates the password, and revokes sessions.
func (h *AuthHandler) PasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil || h.Users == nil || h.PasswordResets == nil {
		respondError(w, http.StatusServiceUnavailable, "password reset unavailable")
		return
	}

	var req passwordResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	userID, err := h.PasswordResets.Consume(r.Context(), hashResetToken(req.Token), time.Now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrPasswordResetTokenInvalid) {
			h.auditSession(r, domain.AuditPasswordResetCompleted, nil, map[string]interface{}{"result": "invalid_token"})
			respondError(w, http.StatusBadRequest, "invalid or expired token")
			return
		}
		h.auditSession(r, domain.AuditPasswordResetCompleted, nil, map[string]interface{}{"result": "internal_error"})
		respondError(w, http.StatusInternalServerError, "reset failed")
		return
	}

	passwordHash, err := h.Auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "reset failed")
		return
	}
	if err := h.Users.UpdatePasswordHash(r.Context(), userID, passwordHash); err != nil {
		respondError(w, http.StatusInternalServerError, "reset failed")
		return
	}
	_, _ = h.Users.RevokeSessions(r.Context(), userID)

	h.clearSessionCookie(w, r)
	h.auditSession(r, domain.AuditPasswordResetCompleted, &userID, map[string]interface{}{"result": "ok"})
	respondJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

// EmailVerificationRequest handles POST /api/auth/email-verification/request.
// It creates a new verification token for the current authenticated user.
func (h *AuthHandler) EmailVerificationRequest(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if u.EmailVerifiedAt != nil {
		respondJSON(w, http.StatusOK, map[string]string{"status": "already_verified"})
		return
	}
	if err := h.enqueueEmailVerification(r, u); err != nil {
		if errors.Is(err, errEmailVerificationUnavailable) {
			respondError(w, http.StatusServiceUnavailable, "email verification unavailable")
			return
		}
		respondError(w, http.StatusInternalServerError, "request failed")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// EmailVerificationConfirm handles POST /api/auth/email-verification/confirm.
// It consumes a one-time token and marks the user's email as verified.
func (h *AuthHandler) EmailVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	if h.Users == nil || h.EmailVerifications == nil {
		respondError(w, http.StatusServiceUnavailable, "email verification unavailable")
		return
	}

	var req emailVerificationConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	now := time.Now().UTC()
	userID, err := h.EmailVerifications.Consume(r.Context(), hashAuthToken(req.Token), now)
	if err != nil {
		if errors.Is(err, domain.ErrEmailVerificationTokenInvalid) {
			h.auditSession(r, domain.AuditEmailVerificationCompleted, nil, map[string]interface{}{"result": "invalid_token"})
			respondError(w, http.StatusBadRequest, "invalid or expired token")
			return
		}
		h.auditSession(r, domain.AuditEmailVerificationCompleted, nil, map[string]interface{}{"result": "internal_error"})
		respondError(w, http.StatusInternalServerError, "verification failed")
		return
	}
	if err := h.Users.UpdateEmailVerifiedAt(r.Context(), userID, &now); err != nil {
		respondError(w, http.StatusInternalServerError, "verification failed")
		return
	}

	h.auditSession(r, domain.AuditEmailVerificationCompleted, &userID, map[string]interface{}{"result": "ok"})
	respondJSON(w, http.StatusOK, map[string]string{"status": "email_verified"})
}

var errEmailVerificationUnavailable = errors.New("email verification unavailable")

func (h *AuthHandler) enqueueEmailVerification(r *http.Request, u *domain.User) error {
	if u == nil || u.EmailVerifiedAt != nil {
		return nil
	}
	if h.Users == nil || h.EmailVerifications == nil || h.Email == nil || h.Queue == nil {
		return errEmailVerificationUnavailable
	}
	if !h.Email.Configured(r.Context()) || strings.TrimSpace(h.AppURL) == "" {
		return errEmailVerificationUnavailable
	}

	rawToken, err := newAuthToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	_ = h.EmailVerifications.DeleteForUser(r.Context(), u.ID)
	if err := h.EmailVerifications.Create(r.Context(), u.ID, hashAuthToken(rawToken), expiresAt, now); err != nil {
		return err
	}

	err = h.Queue.EnqueueEmail(r.Context(), queue.EmailTaskPayload{
		TemplateKey: "email-verification",
		To:          u.Email,
		Vars: map[string]string{
			"Name":      u.Name,
			"AppName":   "Reform Lab",
			"VerifyURL": buildEmailVerificationURL(h.AppURL, rawToken),
			"Year":      fmt.Sprintf("%d", time.Now().Year()),
		},
	}, queue.TaskOptions{MaxRetries: 3, Timeout: 30 * time.Second})
	if err != nil {
		return err
	}

	h.auditSession(r, domain.AuditEmailVerificationRequested, &u.ID, map[string]interface{}{"email": u.Email, "result": "enqueued"})
	return nil
}

func newResetToken() (string, error) {
	return newAuthToken()
}

func newAuthToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashResetToken(raw string) string {
	return hashAuthToken(raw)
}

func hashAuthToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func buildPasswordResetURL(appURL string, token string) string {
	parsed, err := url.Parse(strings.TrimSpace(appURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(appURL, "/") + "/acceso?mode=reset&token=" + url.QueryEscape(token)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/acceso"
	q := parsed.Query()
	q.Set("mode", "reset")
	q.Set("token", token)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func buildEmailVerificationURL(appURL string, token string) string {
	parsed, err := url.Parse(strings.TrimSpace(appURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(appURL, "/") + "/acceso?mode=verify&token=" + url.QueryEscape(token)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/acceso"
	q := parsed.Query()
	q.Set("mode", "verify")
	q.Set("token", token)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func (h *AuthHandler) writeSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r, h.TrustProxyHeaders),
		MaxAge:   sessionCookieMaxAgeSeconds,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r, h.TrustProxyHeaders),
		MaxAge:   -1,
	})
}

func requestUsesHTTPS(r *http.Request, trustProxyHeaders bool) bool {
	if r.TLS != nil {
		return true
	}
	return trustProxyHeaders && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func (h *AuthHandler) auditSession(r *http.Request, eventType domain.AuditEventType, userID *uuid.UUID, details map[string]interface{}) {
	if h.Audit == nil {
		return
	}
	eventDetails := map[string]interface{}{
		"path": r.URL.Path,
	}
	if userID != nil {
		eventDetails["userId"] = userID.String()
	}
	for key, value := range details {
		eventDetails[key] = value
	}
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: eventType,
		Details:   eventDetails,
		CreatedAt: time.Now().UTC(),
	})
}
