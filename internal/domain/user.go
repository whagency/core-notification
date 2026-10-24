package domain

import (
	"context"

	"github.com/whagency/core-common/services"
	"github.com/whagency/core-common/services/user"
)

// UserService defines the interface for validating user sessions.
type UserService interface {
	ValidateSession(context.Context, string, bool) (user.Session, error)
}

// UserServiceGRPC defines the interface for retrieving user profiles via gRPC.
type UserServiceGRPC interface {
	services.GRPCClientBase
	UserService

	GetProfile(context.Context, uint64) (user.Profile, error)
}
