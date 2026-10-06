package users

import "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/constants"

const (
	PackageName   = "user"
	ServiceName   = "UserService"
	FullService   = "/user.UserService"
	JwtPayloadKey = "jwt-payload"
)

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Role        string `json:"role"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	IsActive    bool   `json:"isActive"`
	IsVerified  bool   `json:"isVerified"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type AuthenticatedUser struct {
	UserID    string `json:"userId"`
	Role      string `json:"role"`
	SessionID string `json:"sessionId,omitempty"`
}

type JwtPayload struct {
	UserID    string `json:"userId,omitempty"`
	Sub       string `json:"sub,omitempty"`
	ID        string `json:"id,omitempty"`
	Role      string `json:"role"`
	Email     string `json:"email,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
}

func (p JwtPayload) UserIDOrEmpty() string {
	if p.UserID != "" {
		return p.UserID
	}
	if p.Sub != "" {
		return p.Sub
	}
	return p.ID
}

type PaginationInput struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func NewPaginationInput(page, limit int) PaginationInput {
	if page < 1 {
		page = constants.DefaultPage
	}
	if limit < 1 {
		limit = constants.DefaultLimit
	}
	return PaginationInput{Page: page, Limit: limit}
}

func (p PaginationInput) Offset() int {
	return (p.Page - 1) * p.Limit
}

type GetUserRequest struct {
	ID string `json:"id"`
}

type GetUserByEmailRequest struct {
	Email string `json:"email"`
}

type GetUserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsActive  bool   `json:"is_active"`
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type ValidateTokenResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type GetUserPermissionsRequest struct {
	UserID string `json:"user_id"`
}

type GetUserPermissionsResponse struct {
	Permissions []string `json:"permissions"`
}
