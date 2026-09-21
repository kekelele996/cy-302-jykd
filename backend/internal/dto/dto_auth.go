package dto

import "time"

// RegisterRequest is the payload used when creating an account.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32,alphanum"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Name     string `json:"name" binding:"required,max=64"`
	Role     string `json:"role" binding:"required,oneof=student teacher"`
}

// LoginRequest is the payload used for authentication.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UserProfile is the safe representation of a user.
type UserProfile struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse contains a JWT token and profile.
type LoginResponse struct {
	Token string      `json:"token"`
	User  UserProfile `json:"user"`
}
