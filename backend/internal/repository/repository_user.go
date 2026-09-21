package repository

import (
	"context"
	"fmt"

	"github.com/gbexam/online-exam/internal/model"
)

// CreateUser inserts a new user.
func (r *Repository) CreateUser(ctx context.Context, u *model.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isDuplicateKey(err) {
			return ErrConflict
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindUserByUsername returns a user by unique username.
func (r *Repository) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, wrapQuery("find user by username", err)
	}
	return &u, nil
}

// FindUserByID returns a user by primary key.
func (r *Repository) FindUserByID(ctx context.Context, id uint) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).First(&u, id).Error
	if err != nil {
		return nil, wrapQuery("find user by id", err)
	}
	return &u, nil
}

// ListUsers returns a page of users matching the optional filters.
func (r *Repository) ListUsers(ctx context.Context, keyword, role string, page, pageSize int) ([]model.User, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR name LIKE ?", like, like)
	}
	if role != "" {
		q = q.Where("role = ?", role)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var users []model.User
	p, ps := NormalizePage(page, pageSize)
	if err := q.Order("id DESC").Limit(ps).Offset((p - 1) * ps).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

// UpdateUserStatus enables or disables a user.
func (r *Repository) UpdateUserStatus(ctx context.Context, id uint, status string) error {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return fmt.Errorf("update user status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CountUsers returns the total number of users.
func (r *Repository) CountUsers(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}
