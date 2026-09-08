// Code generated manually from user.proto.
// This package mirrors the proto definitions for internal use.

package proto

import (
	"context"

	"google.golang.org/grpc"
)

// ─── Messages ──────────────────────────────────────────────────────────────

type UpdateUserRoleRequest struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
}

type UpdateUserRoleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ─── Client Interface ──────────────────────────────────────────────────────

type UserServiceClient interface {
	UpdateUserRole(ctx context.Context, in *UpdateUserRoleRequest, opts ...grpc.CallOption) (*UpdateUserRoleResponse, error)
}

type userServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewUserServiceClient(cc grpc.ClientConnInterface) UserServiceClient {
	return &userServiceClient{cc}
}

func (c *userServiceClient) UpdateUserRole(ctx context.Context, in *UpdateUserRoleRequest, opts ...grpc.CallOption) (*UpdateUserRoleResponse, error) {
	out := new(UpdateUserRoleResponse)
	err := c.cc.Invoke(ctx, "/user.UserService/UpdateUserRole", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
