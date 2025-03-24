package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/nasik90/gophermart/internal/app/logger"
	"github.com/nasik90/gophermart/internal/app/storage"
	"github.com/phedde/luhn-algorithm"
	"go.uber.org/zap"
)

type Repository interface {
	SaveNewUser(ctx context.Context, user, password string) error
	UserIsValid(ctx context.Context, login, password string) (bool, error)
	SaveNewOrder(ctx context.Context, orderNumber int, login string) error
	GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error)
	WithdrawPoints(ctx context.Context, login string, orderId int, points float32) error
	AccruePoints(ctx context.Context, orderId int, points float32) error
}

var (
	ErrOrderFormat = errors.New("order format is not valid")
)

type Service struct {
	repo     Repository
	ordersCh chan int
}

func NewService(store Repository) *Service {
	return &Service{repo: store, ordersCh: make(chan int)}
}

func (s *Service) RegisterNewUser(ctx context.Context, login, password string) error {
	return s.repo.SaveNewUser(ctx, login, password)
}

func (s *Service) UserIsValid(ctx context.Context, login, password string) (bool, error) {
	return s.repo.UserIsValid(ctx, login, password)
}

func (s *Service) LoadOrder(ctx context.Context, orderID int, login string) error {
	isValid := luhn.IsValid(int64(orderID))
	if !isValid {
		return ErrOrderFormat
	}
	if err := s.repo.SaveNewOrder(ctx, orderID, login); err != nil {
		return err
	}
	go s.loadOrderIDInOrderQueue(orderID)
	return nil
}

func (s *Service) GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error) {
	return s.repo.GetOrderList(ctx, login)
}

func (s *Service) WithdrawPoints(ctx context.Context, login string, orderId int, points float32) error {
	isValid := luhn.IsValid(int64(orderId))
	if !isValid {
		return ErrOrderFormat
	}
	return s.repo.WithdrawPoints(ctx, login, orderId, points)
}

func (s *Service) loadOrderIDInOrderQueue(orderID int) {
	s.ordersCh <- orderID
}

func (s *Service) HandleOrderQueue(serverAddress string) {
	const (
		status_PROCESSED = "PROCESSED"
	)
	for orderID := range s.ordersCh {
		points, status, err := GetAccrualByOrderID(orderID, serverAddress)
		if err != nil {
			logger.Log.Error("accural api handle", zap.String("error", err.Error()))
		}
		if status == status_PROCESSED {
			s.repo.AccruePoints(context.Background(), orderID, points)
		}
	}
}

func GetAccrualByOrderID(orderID int, serverAddress string) (float32, string, error) {
	client := &http.Client{}
	url := serverAddress + "/api/accrual/" + strconv.Itoa(orderID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	type orderDataType struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float32 `json:"accrual"`
	}
	var orderData orderDataType
	if err := json.NewDecoder(response.Body).Decode(&orderData); err != nil {
		return 0, "", err
	}
	return orderData.Accrual, orderData.Status, nil
}
