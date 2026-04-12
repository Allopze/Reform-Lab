package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/api/middleware"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/rs/zerolog"
)

const sessionCookieName = "reform_session"
const sessionCookieMaxAgeSeconds = 72 * 60 * 60

// AuthHandler handles POST /api/auth/register and /api/auth/login.
type AuthHandler struct {
	Auth   *auth.Service
	Email  *email.Service
	Queue  queue.JobQueue
	Logger zerolog.Logger
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

	writeSessionCookie(w, r, result.SessionToken)

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
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	writeSessionCookie(w, r, result.SessionToken)
	respondJSON(w, http.StatusOK, result)
}

// Logout clears the current session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r)
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

func writeSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r),
		MaxAge:   sessionCookieMaxAgeSeconds,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r),
		MaxAge:   -1,
	})
}

func requestUsesHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}
