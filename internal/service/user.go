package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ilhamazhar/golang-gpt/internal/domain"
)

type userService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) domain.UserService {
	return &userService{repo: repo}
}

func (s *userService) FindAll(ctx context.Context, page, limit int) ([]domain.UserResponse, int64, error) {
	users, total, err := s.repo.FindAll(ctx, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch users: %w", err)
	}
	result := make([]domain.UserResponse, len(users))
	for i, u := range users {
		result[i] = domain.ToUserResponse(&u)
	}
	return result, total, nil
}

func (s *userService) FindByID(ctx context.Context, id uuid.UUID) (domain.UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.UserResponse{}, errors.New("user not found")
	}
	return domain.ToUserResponse(user), nil
}

func (s *userService) Update(ctx context.Context, id uuid.UUID, req domain.UpdateUserRequest) (domain.UserResponse, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.UserResponse{}, errors.New("user not found")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return domain.UserResponse{}, errors.New("email already in use")
	}
	return domain.ToUserResponse(user), nil
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return errors.New("user not found")
		}
		return err
	}
	return nil
}
