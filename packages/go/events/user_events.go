package events

import "time"

// UserEventType represents user lifecycle events
type UserEventType string

const (
	UserCreated UserEventType = "user.created"
	UserUpdated UserEventType = "user.updated"
	UserDeleted UserEventType = "user.deleted"
)

// UserCreatedPayload is emitted when a new user registers
type UserCreatedPayload struct {
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// UserUpdatedPayload is emitted when a user profile is updated
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

// UserDeletedPayload is emitted when a user is deleted
type UserDeletedPayload struct {
	UserID    string    `json:"userId"`
	DeletedAt time.Time `json:"deletedAt"`
}