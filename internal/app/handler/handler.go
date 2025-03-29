package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	middleware "github.com/nasik90/gophermart/internal/app/middlewares"
	"github.com/nasik90/gophermart/internal/app/service"
	"github.com/nasik90/gophermart/internal/app/storage"
)

type Service interface {
	RegisterNewUser(ctx context.Context, user, password string) error
	UserIsValid(ctx context.Context, login, password string) (bool, error)
	LoadOrder(ctx context.Context, orderNumber int, login string) error
	GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error)
	WithdrawPoints(ctx context.Context, login string, OrderID int, points float32) error
	GetUserBalance(ctx context.Context, login string) (*storage.UserBalance, error)
	GetWithdrawals(ctx context.Context, login string) (*[]storage.Withdrawals, error)
}

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterNewUser() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.service.RegisterNewUser(ctx, input.Login, input.Password); err != nil {
			status := http.StatusInternalServerError
			if err == storage.ErrUserNotUnique {
				status = http.StatusConflict
			}
			http.Error(res, err.Error(), status)
			return
		}
		if err := middleware.SetAuthCookie(input.Login, res); err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		res.Header().Set("content-type", "text/plain")
		res.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) LoginUser() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		var input struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		isValid, err := h.service.UserIsValid(ctx, input.Login, input.Password)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if !isValid {
			http.Error(res, "", http.StatusUnauthorized)
			return
		}
		if err := middleware.SetAuthCookie(input.Login, res); err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		res.Header().Set("content-type", "text/plain")
		res.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) LoadOrder() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var buf bytes.Buffer
		ctx := req.Context()
		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		orderNumber := buf.String()
		if orderNumber == "" {
			http.Error(res, "empty url", http.StatusBadRequest)
			return
		}
		login := middleware.LoginFromContext(ctx)
		orderNumberInt, err := strconv.Atoi(orderNumber)
		if err != nil {
			http.Error(res, service.ErrOrderFormat.Error(), http.StatusUnprocessableEntity)
			return
		}
		err = h.service.LoadOrder(ctx, orderNumberInt, login)
		resStatus := http.StatusAccepted
		if err != nil {
			if err == storage.ErrOrderLoadedByAnotherUser {
				http.Error(res, err.Error(), http.StatusConflict)
				return
			} else if err == service.ErrOrderFormat {
				http.Error(res, err.Error(), http.StatusUnprocessableEntity)
				return
			} else if err == storage.ErrOrderIDNotUnique {
				resStatus = http.StatusOK
			} else {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		res.Header().Set("content-type", "text/plain")
		res.WriteHeader(resStatus)
	}
}

func (h *Handler) GetOrderList() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		login := middleware.LoginFromContext(ctx)
		orderList, err := h.service.GetOrderList(ctx, login)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		resStatus := http.StatusNoContent
		var orderListJSON []byte
		if len(*orderList) != 0 {
			orderListJSON, err = json.Marshal(orderList)
			if err != nil {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
			resStatus = http.StatusOK
		}
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(resStatus)
		res.Write(orderListJSON)

		// res.Header().Set("content-type", "application/json")
		// res.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) WithdrawPoints() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		login := middleware.LoginFromContext(ctx)
		var input struct {
			Order string  `json:"order"`
			Sum   float32 `json:"sum"`
		}
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		orderNumberInt, err := strconv.Atoi(input.Order)
		if err != nil {
			http.Error(res, service.ErrOrderFormat.Error(), http.StatusUnprocessableEntity)
			return
		}
		err = h.service.WithdrawPoints(ctx, login, orderNumberInt, input.Sum)
		resStatus := http.StatusOK
		if err != nil {
			if err == storage.ErrOrderLoadedByAnotherUser {
				http.Error(res, err.Error(), http.StatusConflict)
				return
			} else if err == service.ErrOrderFormat {
				http.Error(res, err.Error(), http.StatusUnprocessableEntity)
				return
			} else if err == storage.ErrOrderIDNotUnique {
				http.Error(res, err.Error(), http.StatusConflict)
				return
			} else if err == storage.ErrOutOfBalance {
				http.Error(res, err.Error(), http.StatusPaymentRequired)
				return
			} else {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		res.Header().Set("content-type", "text/plain")
		res.WriteHeader(resStatus)
	}
}

func (h *Handler) GetAccrual() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		type resType struct {
			Order   string  `json:"order"`
			Status  string  `json:"status"`
			Accrual float32 `json:"accrual"`
		}
		var result resType
		result.Order = "123"
		result.Status = "PROCESSED"
		result.Accrual = 500
		var orderListJSON []byte
		orderListJSON, err := json.Marshal(result)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		resStatus := http.StatusOK
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(resStatus)
		res.Write(orderListJSON)
	}
}

func (h *Handler) GetUserBalance() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		login := middleware.LoginFromContext(ctx)
		userBalance, err := h.service.GetUserBalance(ctx, login)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		var userBalanceJSON []byte
		userBalanceJSON, err = json.Marshal(userBalance)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(userBalanceJSON)
	}
}

func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		login := middleware.LoginFromContext(ctx)
		orderList, err := h.service.GetWithdrawals(ctx, login)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		resStatus := http.StatusNoContent
		var orderListJSON []byte
		if len(*orderList) != 0 {
			orderListJSON, err = json.Marshal(orderList)
			if err != nil {
				http.Error(res, err.Error(), http.StatusInternalServerError)
				return
			}
			resStatus = http.StatusOK
		}
		res.Header().Set("content-type", "application/json")
		res.WriteHeader(resStatus)
		res.Write(orderListJSON)

		// res.Header().Set("content-type", "application/json")
		// res.WriteHeader(http.StatusOK)
	}
}
