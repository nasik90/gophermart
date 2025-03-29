package storage

import (
	"errors"
	"time"
)

var (
	ErrUserNotUnique            = errors.New("user is not unique")
	ErrOrderIDNotUnique         = errors.New("order id is not unique")
	ErrOrderLoadedByAnotherUser = errors.New("order loaded by another user")
	ErrOutOfBalance             = errors.New("out of balance")
)

type OrderData struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type UserBalance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdrawals struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
