package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

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
	WithdrawPoints(ctx context.Context, login string, OrderID int, points float64) error
	AccruePoints(ctx context.Context, OrderID int, points float64) error
	GetUserBalance(ctx context.Context, login string) (*storage.UserBalance, error)
	GetWithdrawals(ctx context.Context, login string) (*[]storage.Withdrawals, error)
	SaveStatusInvalid(ctx context.Context, orderID int) error
	SaveStatusProccessing(ctx context.Context, orderID int) error
}

var (
	ErrOrderFormat     = errors.New("order format is not valid")
	ErrTooManyRequests = errors.New("too many requests")
)

type Service struct {
	repo        Repository
	ordersCh    chan int
	badOrdersCh chan int
}

func NewService(store Repository) *Service {
	return &Service{repo: store, ordersCh: make(chan int), badOrdersCh: make(chan int)}
}

func (s *Service) RegisterNewUser(ctx context.Context, login, password string) error {
	return s.repo.SaveNewUser(ctx, login, password)
}

func (s *Service) UserIsValid(ctx context.Context, login, password string) (bool, error) {
	return s.repo.UserIsValid(ctx, login, password)
}

func (s *Service) LoadOrder(ctx context.Context, OrderID int, login string) error {
	isValid := luhn.IsValid(int64(OrderID))
	if !isValid {
		return ErrOrderFormat
	}
	if err := s.repo.SaveNewOrder(ctx, OrderID, login); err != nil {
		return err
	}
	go s.loadOrderIDInOrderQueue(OrderID)
	return nil
}

func (s *Service) GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error) {
	return s.repo.GetOrderList(ctx, login)
}

// списание баллов
func (s *Service) WithdrawPoints(ctx context.Context, login string, OrderID int, points float64) error {
	isValid := luhn.IsValid(int64(OrderID))
	if !isValid {
		return ErrOrderFormat
	}
	return s.repo.WithdrawPoints(ctx, login, OrderID, points)
}

func (s *Service) loadOrderIDInOrderQueue(orderID int) {
	s.ordersCh <- orderID
}

func (s *Service) HandleOrderQueue(serverAddress string) {
	const (
		statusREGISTERED = "REGISTERED"
		statusINVALID    = "INVALID"
		statusPROCESSING = "PROCESSING"
		statusPROCESSED  = "PROCESSED"
	)
	ctx := context.Background()
	//orderIDsForRepeat := []int
	for {
		orderID := 0
		select {
		case orderID = <-s.ordersCh:
			points, status, err := GetAccrualByOrderID(orderID, serverAddress)
			if err == ErrTooManyRequests {
				time.Sleep(3 * time.Second)
				s.badOrdersCh <- orderID
			}
			if err != nil {
				logger.Log.Error("accural api handle", zap.String("error", err.Error()))
			}
			switch status {
			case statusREGISTERED:
				s.badOrdersCh <- orderID
			case statusINVALID:
				if err := s.repo.SaveStatusInvalid(ctx, orderID); err != nil {
					logger.Log.Error("status handling error", zap.String("status", statusINVALID), zap.String("error", err.Error()))
					s.badOrdersCh <- orderID
				}
			case statusPROCESSING:
				if err := s.repo.SaveStatusProccessing(ctx, orderID); err != nil {
					logger.Log.Error("status handling error", zap.String("status", statusINVALID), zap.String("error", err.Error()))
				}
				s.badOrdersCh <- orderID
			case statusPROCESSED:
				if err := s.repo.AccruePoints(ctx, orderID, points); err != nil {
					logger.Log.Error("status handling error", zap.String("status", statusINVALID), zap.String("error", err.Error()))
					//orderIDsForRepeat = append(orderIDsForRepeat, orderID)
					s.badOrdersCh <- orderID
				}
			}
		}
	}
	//<-s.ordersCh
}

func (s *Service) HandleBadOrdersQueue() {
	for {
		select {
		case badOrderID := <-s.badOrdersCh:
			s.ordersCh <- badOrderID
		}
	}
}

func GetAccrualByOrderID(orderID int, serverAddress string) (float64, string, error) {
	//return 0, "", nil
	start := time.Now()
	client := &http.Client{}
	url := "http://" + serverAddress + "/api/accrual/" + strconv.Itoa(orderID)
	//url := serverAddress + "/api/accrual/" + strconv.Itoa(orderID)
	logger.Log.Info("accural handle", zap.String("api url", url))
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		logger.Log.Fatal("accural request init", zap.String("error", err.Error()))
	}
	response, err := client.Do(request)
	if err != nil {
		logger.Log.Fatal("accural request do", zap.String("error", err.Error()))
	}
	defer response.Body.Close()
	duration := time.Since(start)
	logger.Log.Sugar().Infoln(
		"uri", request.URL.Path,
		"method", request.Method,
		"status", response.StatusCode,
		"duration", duration,
	)
	if response.StatusCode == http.StatusTooManyRequests {
		return 0.0, "", ErrTooManyRequests
	}
	type orderDataType struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float64 `json:"accrual"`
	}
	var orderData orderDataType
	if err := json.NewDecoder(response.Body).Decode(&orderData); err != nil {
		b, errReadAll := io.ReadAll(response.Body)
		if errReadAll != nil {
			logger.Log.Fatal("fatal accural handle body read", zap.String("error", err.Error()))
		}
		logger.Log.Error("accural handle response", zap.String("response body", string(b)))
		return 0.0, "", err
	}
	return orderData.Accrual, orderData.Status, nil
}

func (s *Service) GetUserBalance(ctx context.Context, login string) (*storage.UserBalance, error) {
	return s.repo.GetUserBalance(ctx, login)
}

// список списаний
func (s *Service) GetWithdrawals(ctx context.Context, login string) (*[]storage.Withdrawals, error) {
	return s.repo.GetWithdrawals(ctx, login)
}
