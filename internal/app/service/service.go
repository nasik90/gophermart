package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	SaveStatus(ctx context.Context, orderID, statusID int) error
	NewAndProcessingOrders(ctx context.Context) ([]int, error)
}

var (
	ErrOrderFormat        = errors.New("order format is not valid")
	ErrTooManyRequests    = errors.New("too many requests")
	ErrOrderNotRegistered = errors.New("order not registered")
)

type Service struct {
	repo         Repository
	ordersCh     chan int
	checkOrderID bool
}

func NewService(store Repository, checkOrderID bool) *Service {
	return &Service{repo: store, ordersCh: make(chan int), checkOrderID: checkOrderID}
}

func (s *Service) RegisterNewUser(ctx context.Context, login, password string) error {
	return s.repo.SaveNewUser(ctx, login, password)
}

func (s *Service) UserIsValid(ctx context.Context, login, password string) (bool, error) {
	return s.repo.UserIsValid(ctx, login, password)
}

func (s *Service) LoadOrder(ctx context.Context, OrderID int, login string) error {
	if s.checkOrderID {
		isValid := luhn.IsValid(int64(OrderID))
		if !isValid {
			return ErrOrderFormat
		}
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
	if s.checkOrderID {
		isValid := luhn.IsValid(int64(OrderID))
		if !isValid {
			return ErrOrderFormat
		}
	}
	return s.repo.WithdrawPoints(ctx, login, OrderID, points)
}

func (s *Service) loadOrderIDInOrderQueue(orderID int) {
	s.ordersCh <- orderID
}

func (s *Service) HandleOrderQueue(serverAddress string, stop <-chan bool) {

	ctx := context.Background()
	forIter := 0
	for {
		select {
		default:
			orderIDs, err := s.repo.NewAndProcessingOrders(ctx)
			if err != nil {
				logger.Log.Error("select orders for processing in accrual service", zap.String("error", err.Error()))
				return
			}
			if err := s.handleOrders(ctx, orderIDs, serverAddress); err != nil {
				logger.Log.Error("order handle via accrual", zap.String("error", err.Error()))
			}
			// Если нет заказов для обработки, то сделаем паузу
			// Пауза равна от 1 по нарастающей, максимум 3 секунды
			if len(orderIDs) == 0 {
				forIter++
				time.Sleep(time.Duration(forIter) * time.Second)
				if forIter == 3 {
					forIter = 0
				}
			} else {
				forIter = 0
			}
		case <-stop:
			return
		}
	}
}

func (s *Service) handleOrders(ctx context.Context, orderIDs []int, serverAddress string) error {
	const (
		statusREGISTERED = "REGISTERED"
		statusINVALID    = "INVALID"
		statusPROCESSING = "PROCESSING"
		statusPROCESSED  = "PROCESSED"
	)
	for _, orderID := range orderIDs {
		accrualData, retryAfter, err := GetAccrualByOrderID(orderID, serverAddress)
		if errors.Is(err, ErrTooManyRequests) {
			timeToSleep := 5
			if retryAfter != 0 {
				timeToSleep = retryAfter
			}
			time.Sleep(time.Second * time.Duration(timeToSleep))
		}
		if errors.Is(err, ErrOrderNotRegistered) {
			continue
		}
		if err != nil {
			return err
		}

		switch accrualData.Status {
		case statusREGISTERED:
		case statusINVALID:
			if err := s.repo.SaveStatus(ctx, orderID, storage.StatusINVALID); err != nil {
				return errors.Join(errors.New("status: "+statusINVALID), err)
			}
		case statusPROCESSING:
			if err := s.repo.SaveStatus(ctx, orderID, storage.StatusPROCESSING); err != nil {
				return errors.Join(errors.New("status: "+statusPROCESSING), err)
			}
		case statusPROCESSED:
			if err := s.repo.AccruePoints(ctx, orderID, accrualData.Accrual); err != nil {
				return errors.Join(errors.New("status: "+statusPROCESSED), err)
			}
		}
	}
	return nil
}

type orderDataType struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

func GetAccrualByOrderID(orderID int, serverAddress string) (orderDataType, int, error) {
	start := time.Now()
	var orderData orderDataType
	client := &http.Client{}
	// Как сделать красиво?
	serverPrefix := ""
	if !strings.Contains(serverAddress, "http") {
		serverPrefix = "http://"
	}
	url := serverPrefix + serverAddress + "/api/orders/" + strconv.Itoa(orderID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return orderData, 0, err
	}
	response, err := client.Do(request)
	if err != nil {
		return orderData, 0, err
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
		retryAfterString := response.Header.Get("Retry-After")
		retryAfter, err := strconv.Atoi(retryAfterString)
		if err != nil {
			return orderData, 0, err
		}
		return orderData, retryAfter, ErrTooManyRequests
	}
	if response.StatusCode == http.StatusNoContent {
		return orderData, 0, ErrOrderNotRegistered
	}

	if err := json.NewDecoder(response.Body).Decode(&orderData); err != nil {
		return orderData, 0, err
	}
	return orderData, 0, nil
}

func (s *Service) GetUserBalance(ctx context.Context, login string) (*storage.UserBalance, error) {
	return s.repo.GetUserBalance(ctx, login)
}

// список списаний
func (s *Service) GetWithdrawals(ctx context.Context, login string) (*[]storage.Withdrawals, error) {
	return s.repo.GetWithdrawals(ctx, login)
}
