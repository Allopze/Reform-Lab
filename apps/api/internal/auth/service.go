package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const tokenExpiry = 72 * time.Hour

// Claims is the JWT payload.
type Claims struct {
	Role           domain.UserRole `json:"role"`
	SessionVersion int             `json:"sessionVersion"`
	jwt.RegisteredClaims
}

func (c Claims) Validate() error {
	if c.Role != domain.RoleAdmin && c.Role != domain.RoleUser {
		return errors.New("invalid role in token")
	}
	if c.SessionVersion < 1 {
		return errors.New("invalid session version in token")
	}
	return nil
}

// Service handles registration and authentication.
type Service struct {
	users                    repository.UserRepository
	jwtSecret                []byte
	requireExplicitBootstrap bool
	bootstrapAdminEmails     map[string]struct{}
}

func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Option customizes the auth service.
type Option func(*Service)

// NewService creates an auth service.
func NewService(users repository.UserRepository, jwtSecret string, opts ...Option) *Service {
	svc := &Service{users: users, jwtSecret: []byte(jwtSecret)}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// WithExplicitBootstrapRequired requires the first admin to be bootstrapped
// explicitly instead of being granted to the first public registrant.
func WithExplicitBootstrapRequired(required bool) Option {
	return func(s *Service) {
		s.requireExplicitBootstrap = required
	}
}

// WithBootstrapAdminEmails allowlists the email addresses that may claim the
// initial admin bootstrap when no users exist yet.
func WithBootstrapAdminEmails(emails []string) Option {
	return func(s *Service) {
		if len(emails) == 0 {
			s.bootstrapAdminEmails = nil
			return
		}
		allowed := make(map[string]struct{}, len(emails))
		for _, email := range emails {
			if normalized := normalizeEmail(email); normalized != "" {
				allowed[normalized] = struct{}{}
			}
		}
		if len(allowed) == 0 {
			s.bootstrapAdminEmails = nil
			return
		}
		s.bootstrapAdminEmails = allowed
	}
}

// RegisterInput carries validated registration data.
type RegisterInput struct {
	Name     string
	Email    string
	Team     string
	Password string
}

// AuthResult is returned on successful register or login.
type AuthResult struct {
	User         *domain.User `json:"user"`
	SessionToken string       `json:"-"`
}

// Register creates a new user account and returns a JWT.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	role := domain.RoleUser

	hasAdmin, err := s.users.HasAdmin(ctx)
	if err != nil {
		return nil, err
	}

	if !hasAdmin {
		switch {
		case s.canBootstrapInitialAdmin(in.Email):
			role = domain.RoleAdmin
		case len(s.bootstrapAdminEmails) > 0 && s.requireExplicitBootstrap:
			return nil, domain.ErrBootstrapAdminRequired
		default:
			role = domain.RoleAdmin
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		ID:             uuid.New(),
		Name:           in.Name,
		Email:          in.Email,
		Team:           in.Team,
		Role:           role,
		SessionVersion: 1,
		PasswordHash:   string(hash),
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.users.Create(ctx, u); err != nil {
		if role == domain.RoleAdmin && !errors.Is(err, domain.ErrEmailAlreadyExists) {
			u.Role = domain.RoleUser
			if retryErr := s.users.Create(ctx, u); retryErr != nil {
				return nil, retryErr
			}
		} else {
			return nil, err
		}
	}

	token, err := s.issueToken(u.ID, u.Role, u.SessionVersion)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: u, SessionToken: token}, nil
}

// Login verifies credentials and returns a JWT.
func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if u.IsSuspended {
		return nil, domain.ErrUserSuspended
	}

	token, err := s.issueToken(u.ID, u.Role, u.SessionVersion)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: u, SessionToken: token}, nil
}

// ValidateToken parses and validates a JWT, returning the user ID.
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (s *Service) issueToken(userID uuid.UUID, role domain.UserRole, sessionVersion int) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Role:           role,
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenExpiry)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Service) canBootstrapInitialAdmin(email string) bool {
	if len(s.bootstrapAdminEmails) == 0 {
		return !s.requireExplicitBootstrap
	}
	_, ok := s.bootstrapAdminEmails[normalizeEmail(email)]
	return ok
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
