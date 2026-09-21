package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gbexam/online-exam/internal/config"
	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
)

// AuthService handles registration, login and token validation.
type AuthService struct {
	baseService
	repo   UserRepo
	cfg    *config.Config
	mu     sync.Mutex
	tokens map[uint]string // userID -> current jti
}

// NewAuthService constructs AuthService.
func NewAuthService(repo UserRepo, cfg *config.Config, logger *slog.Logger) *AuthService {
	return &AuthService{
		baseService: NewBaseService(logger),
		repo:        repo,
		cfg:         cfg,
		tokens:      make(map[uint]string),
	}
}

type authClaims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Register creates a student or teacher account.
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.UserProfile, error) {
	if _, err := s.repo.FindUserByUsername(ctx, req.Username); err == nil {
		return nil, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("find user by username: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Name:         req.Name,
		Role:         req.Role,
		Status:       constants.UserActive,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return profileFromUser(user), nil
}

// Login verifies credentials and returns a JWT.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	if user.Status == constants.UserDisabled {
		return nil, ErrForbidden
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrUnauthorized
	}

	token, jti, err := s.issueToken(user)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.tokens[user.ID] = jti
	s.mu.Unlock()

	return &dto.LoginResponse{Token: token, User: *profileFromUser(user)}, nil
}

// Logout invalidates the current session token.
func (s *AuthService) Logout(ctx context.Context, userID uint) {
	s.mu.Lock()
	delete(s.tokens, userID)
	s.mu.Unlock()
}

// Profile returns the profile of a user id.
func (s *AuthService) Profile(ctx context.Context, userID uint) (*dto.UserProfile, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profileFromUser(user), nil
}

// ParseToken validates a JWT and returns the embedded user identity.
func (s *AuthService) ParseToken(tokenString string) (uint, string, string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &authClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", "", ErrUnauthorized
	}

	claims, ok := token.Claims.(*authClaims)
	if !ok {
		return 0, "", "", ErrUnauthorized
	}
	jti := claims.RegisteredClaims.ID
	s.mu.Lock()
	current, exists := s.tokens[claims.UserID]
	s.mu.Unlock()
	if !exists || current != jti {
		return 0, "", "", ErrUnauthorized
	}
	return claims.UserID, claims.Username, claims.Role, nil
}

// SeedAdmin ensures the preset administrator account exists.
func (s *AuthService) SeedAdmin(ctx context.Context) error {
	_, err := s.repo.FindUserByUsername(ctx, s.cfg.AdminUsername)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("find admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	admin := &model.User{
		Username:     s.cfg.AdminUsername,
		PasswordHash: string(hash),
		Name:         "系统管理员",
		Role:         constants.RoleAdmin,
		Status:       constants.UserActive,
	}
	if err := s.repo.CreateUser(ctx, admin); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func (s *AuthService) issueToken(user *model.User) (string, string, error) {
	jti := fmt.Sprintf("%d-%d", user.ID, time.Now().UnixNano())
	now := time.Now()
	claims := &authClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(s.cfg.JWTExpireHours) * time.Hour)),
			Issuer:    "gbexam",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", "", fmt.Errorf("sign token: %w", err)
	}
	return signed, jti, nil
}

func profileFromUser(user *model.User) *dto.UserProfile {
	return &dto.UserProfile{
		ID:        user.ID,
		Username:  user.Username,
		Name:      user.Name,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
	}
}

// normalizeUsername trims whitespace from credentials.
func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}
