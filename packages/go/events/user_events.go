package events

import "time"

type UserEventType string

const (
	UserCreated UserEventType = "user.created"
	UserUpdated UserEventType = "user.updated"
	UserDeleted UserEventType = "user.deleted"
)

type UserCreatedPayload struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type UserUpdatedPayload struct {
	UserID    string     `json:"userId"`
	Email     string     `json:"email"`
	FirstName string     `json:"firstName"`
	LastName  string     `json:"lastName"`
	Role      string     `json:"role"`
	IsActive  bool       `json:"isActive"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	AvatarID  *string    `json:"avatarMediaId,omitempty"`
}

type UserDeletedPayload struct {
	UserID    string    `json:"userId"`
	DeletedAt time.Time `json:"deletedAt"`
}