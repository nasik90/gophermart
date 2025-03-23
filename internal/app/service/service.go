package service

import (
	"context"
	"errors"

	"github.com/nasik90/gophermart/internal/app/storage"
)

type Repository interface {
	SaveNewUser(ctx context.Context, user, password string) error
	UserIsValid(ctx context.Context, login, password string) (bool, error)
	SaveNewOrder(ctx context.Context, orderNumber int, login string) error
	GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error)
	WithdrawPoints(ctx context.Context, login string, orderId int, points float32) error
}

var (
	ErrOrderFormat = errors.New("order format is not valid")
)

type Service struct {
	repo Repository
}

func NewService(store Repository) *Service {
	return &Service{repo: store}
}

func (s *Service) RegisterNewUser(ctx context.Context, login, password string) error {
	return s.repo.SaveNewUser(ctx, login, password)
}

func (s *Service) UserIsValid(ctx context.Context, login, password string) (bool, error) {
	return s.repo.UserIsValid(ctx, login, password)
}

func (s *Service) LoadOrder(ctx context.Context, orderNumber int, login string) error {
	// isValid := luhn.IsValid(orderNumber)
	// if !isValid {
	// 	return ErrOrderFormat
	// }
	return s.repo.SaveNewOrder(ctx, orderNumber, login)
}

func (s *Service) GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error) {
	return s.repo.GetOrderList(ctx, login)
}

func (s *Service) WithdrawPoints(ctx context.Context, login string, orderId int, points float32) error {
	// isValid := luhn.IsValid(orderNumber)
	// if !isValid {
	// 	return ErrOrderFormat
	// }
	return s.repo.WithdrawPoints(ctx, login, orderId, points)
}
