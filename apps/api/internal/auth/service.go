package auth

import (
	"context"
	"errors"
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
	Role domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func (c Claims) Validate() error {
	if c.Role != domain.RoleAdmin && c.Role != domain.RoleUser {
		return errors.New("invalid role in token")
	}
	return nil
}

// Service handles registration and authentication.
type Service struct {
	users     repository.UserRepository
	jwtSecret []byte
}

// NewService creates an auth service.
func NewService(users repository.UserRepository, jwtSecret string) *Service {
	return &Service{users: users, jwtSecret: []byte(jwtSecret)}
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
	User  *domain.User `json:"user"`
	Token string       `json:"token"`
}

// Register creates a new user account and returns a JWT.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	role := domain.RoleUser
	count, err := s.users.Count(ctx)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		role = domain.RoleAdmin
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		ID:           uuid.New(),
		Name:         in.Name,
		Email:        in.Email,
		Team:         in.Team,
		Role:         role,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
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

	token, err := s.issueToken(u.ID, u.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: u, Token: token}, nil
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

	token, err := s.issueToken(u.ID, u.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: u, Token: token}, nil
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

func (s *Service) issueToken(userID uuid.UUID, role domain.UserRole) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenExpiry)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}
