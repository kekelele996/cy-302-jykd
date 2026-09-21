package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gbexam/online-exam/internal/dto"
)

// UserService handles user administration.
type UserService struct {
	baseService
	repo UserRepo
}

// NewUserService constructs UserService.
func NewUserService(repo UserRepo, logger *slog.Logger) *UserService {
	return &UserService{baseService: NewBaseService(logger), repo: repo}
}

// List returns a page of users.
func (s *UserService) List(ctx context.Context, query dto.UserListQuery) (dto.PageResult, error) {
	users, total, err := s.repo.ListUsers(ctx, query.Keyword, query.Role, query.Page, query.PageSize)
	if err != nil {
		return dto.PageResult{}, fmt.Errorf("list users: %w", err)
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items := make([]dto.UserProfile, 0, len(users))
	for i := range users {
		items = append(items, *profileFromUser(&users[i]))
	}
	return dto.PageResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// UpdateStatus enables or disables a user.
func (s *UserService) UpdateStatus(ctx context.Context, id uint, status string) error {
	if err := s.repo.UpdateUserStatus(ctx, id, status); err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
}

// normalizePage mirrors repository defaults so response metadata is accurate.
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
