package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	middleware "github.com/nasik90/gophermart/internal/app/middlewares"
	mock_service "github.com/nasik90/gophermart/internal/app/mocks"
	"github.com/nasik90/gophermart/internal/app/service"
	"github.com/nasik90/gophermart/internal/app/storage"
	"github.com/stretchr/testify/assert"
)

func TestHandler_RegisterNewUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	type input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	tests := []struct {
		name         string
		input        input
		responseCode int
	}{
		{
			name:         "positive test #1",
			input:        input{Login: "vasiliy", Password: "123"},
			responseCode: http.StatusOK,
		},
		// {
		// 	name:         "negative test #1",
		// 	input:        input{Login: "petya", Password: "123"},
		// 	responseCode: http.StatusConflict,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := httptest.NewRecorder().Body
			inputJSON, _ := json.Marshal(&tt.input)
			body.Write(inputJSON)
			request := httptest.NewRequest(http.MethodPost, "/", body)

			mockRepo.EXPECT().SaveNewUser(request.Context(), tt.input.Login, tt.input.Password).Return(nil).MinTimes(1).MaxTimes(2)

			if tt.responseCode == http.StatusConflict {
				mockRepo.SaveNewUser(context.Background(), tt.input.Login, tt.input.Password)
			}

			w := httptest.NewRecorder()
			h.RegisterNewUser()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}

func TestHandler_LoginUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	type input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	tests := []struct {
		name         string
		input        input
		responseCode int
	}{
		{
			name:         "positive test #1",
			input:        input{Login: "vasiliy", Password: "123"},
			responseCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := httptest.NewRecorder().Body
			inputJSON, _ := json.Marshal(&tt.input)
			body.Write(inputJSON)
			request := httptest.NewRequest(http.MethodPost, "/", body)

			mockRepo.EXPECT().UserIsValid(request.Context(), tt.input.Login, tt.input.Password).Return(true, nil)

			w := httptest.NewRecorder()
			h.LoginUser()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}

func TestHandler_LoadOrder(t *testing.T) {

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	tests := []struct {
		name         string
		orderID      string
		responseCode int
		login        string
	}{
		{
			name:         "positive test #1",
			orderID:      "378282246310005",
			responseCode: http.StatusAccepted,
			login:        "vasya",
		},
		{
			name:         "order format error",
			orderID:      "1789372997",
			responseCode: http.StatusUnprocessableEntity,
			login:        "vasya",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			orderIDint, _ := strconv.Atoi(tt.orderID)

			body := httptest.NewRecorder().Body
			body.Write([]byte(tt.orderID))
			request := httptest.NewRequest(http.MethodPost, "/", body).
				WithContext(context.WithValue(ctx, middleware.LoginContextKey{}, tt.login))

			mockRepo.EXPECT().SaveNewOrder(request.Context(), orderIDint, tt.login).Return(nil)

			w := httptest.NewRecorder()
			h.LoadOrder()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)

			if res.StatusCode == http.StatusUnprocessableEntity {
				mockRepo.SaveNewOrder(request.Context(), orderIDint, tt.login)
			}

		})
	}
}

func TestHandler_GetOrderList(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	tests := []struct {
		name         string
		responseCode int
		login        string
	}{
		{
			name:         "test #1 no data",
			responseCode: http.StatusNoContent,
			login:        "vasya",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			body := httptest.NewRecorder().Body
			request := httptest.NewRequest(http.MethodGet, "/", body).
				WithContext(context.WithValue(ctx, middleware.LoginContextKey{}, tt.login))

			mockRepo.EXPECT().GetOrderList(request.Context(), tt.login).Return(nil, nil)

			w := httptest.NewRecorder()
			h.GetOrderList()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}

func TestHandler_WithdrawPoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	type input struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}

	tests := []struct {
		name         string
		input        input
		login        string
		responseCode int
	}{
		{
			name:         "positive test #1",
			input:        input{Order: "378282246310005", Sum: 450.0},
			login:        "testUser",
			responseCode: http.StatusOK,
		},
		{
			name:         "negative test out of balance",
			input:        input{Order: "378282246310005", Sum: 450.0},
			login:        "testUser",
			responseCode: http.StatusPaymentRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := httptest.NewRecorder().Body
			inputJSON, _ := json.Marshal(&tt.input)
			body.Write(inputJSON)
			request := httptest.NewRequest(http.MethodPost, "/", body).
				WithContext(context.WithValue(context.Background(), middleware.LoginContextKey{}, tt.login))

			orderInt, err := strconv.Atoi(tt.input.Order)
			assert.NoError(t, err)
			if tt.responseCode == http.StatusPaymentRequired {
				mockRepo.EXPECT().WithdrawPoints(request.Context(), tt.login, orderInt, tt.input.Sum).Return(storage.ErrOutOfBalance)
			} else {
				mockRepo.EXPECT().WithdrawPoints(request.Context(), tt.login, orderInt, tt.input.Sum).Return(nil)
			}

			w := httptest.NewRecorder()
			h.WithdrawPoints()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}

func TestHandler_GetUserBalance(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	tests := []struct {
		name         string
		responseCode int
		login        string
	}{
		{
			name:         "test #1 no data",
			responseCode: http.StatusOK,
			login:        "vasya",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			body := httptest.NewRecorder().Body
			request := httptest.NewRequest(http.MethodGet, "/", body).
				WithContext(context.WithValue(ctx, middleware.LoginContextKey{}, tt.login))

			mockRepo.EXPECT().GetUserBalance(request.Context(), tt.login).Return(nil, nil)

			w := httptest.NewRecorder()
			h.GetUserBalance()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}

func TestHandler_GetWithdrawals(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := mock_service.NewMockRepository(ctrl)
	s := service.NewService(mockRepo, true)
	h := NewHandler(s)

	tests := []struct {
		name         string
		responseCode int
		login        string
	}{
		{
			name:         "test #1 no data",
			responseCode: http.StatusNoContent,
			login:        "vasya",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			body := httptest.NewRecorder().Body
			request := httptest.NewRequest(http.MethodGet, "/", body).
				WithContext(context.WithValue(ctx, middleware.LoginContextKey{}, tt.login))

			mockRepo.EXPECT().GetWithdrawals(request.Context(), tt.login).Return(nil, nil)

			w := httptest.NewRecorder()
			h.GetWithdrawals()(w, request)
			res := w.Result()
			res.Body.Close()
			assert.Equal(t, tt.responseCode, res.StatusCode)
		})
	}
}
