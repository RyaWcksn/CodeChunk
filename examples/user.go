// example user service in Go for chunking demos
package user

import (
	"context"
	"errors"
	"strings"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleGuest  Role = "guest"
)

const MaxUsers = 1000

type User struct {
	ID        string
	Email     string
	CreatedAt int64
}

func (u *User) NormalizeEmail() string {
	return strings.ToLower(strings.TrimSpace(u.Email))
}

type Service struct {
	cache map[string]*User
}

func NewService() *Service {
	return &Service{cache: make(map[string]*User)}
}

func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
	if u, ok := s.cache[id]; ok {
		return u, nil
	}
	return s.fetch(ctx, id)
}

func (s *Service) fetch(ctx context.Context, id string) (*User, error) {
	return nil, errors.New("not implemented")
}

func DefaultRole() Role { return RoleMember }
