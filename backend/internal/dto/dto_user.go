package dto

// UserListQuery filters the user list.
type UserListQuery struct {
	PageQuery
	Keyword string `form:"keyword"`
	Role    string `form:"role" binding:"omitempty,oneof=admin teacher student"`
}

// UserStatusRequest enables or disables a user.
type UserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active disabled"`
}
