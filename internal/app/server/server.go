package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nasik90/gophermart/internal/app/handler"
	"github.com/nasik90/gophermart/internal/app/logger"
	middleware "github.com/nasik90/gophermart/internal/app/middlewares"
	"go.uber.org/zap"
)

type Server struct {
	http.Server
	handler *handler.Handler
}

func NewServer(handler *handler.Handler, serverAddress string) *Server {
	s := &Server{}
	s.Addr = serverAddress
	s.handler = handler
	return s
}

func (s *Server) RunServer() error {

	logger.Log.Info("Running server", zap.String("address", s.Addr))

	r := chi.NewRouter()
	// r.Group(func(r chi.Router) {
	// 	r.Use()
	// })
	r.Route("/api", func(r chi.Router) {
		r.Post("/user/register1", s.handler.RegisterNewUser())
		r.Post("/user/login", s.handler.LoginUser())
		r.Post("/user/orders1", middleware.Auth(s.handler.LoadOrder()))
		r.Get("/user/orders", middleware.Auth(s.handler.GetOrderList()))
		// r.Get("/user/balance", middleware.Auth(s.handler.GetOrderList()))
		r.Post("/user/balance/withdraw", middleware.Auth(s.handler.WithdrawPoints()))
		// r.Get("/user/user/withdrawals", middleware.Auth(s.handler.GetOrderList()))
		r.Get("/accrual", s.handler.GetAccrual())
	})
	s.Handler = logger.RequestLogger((middleware.GzipMiddleware(r.ServeHTTP)))
	err := s.ListenAndServe()
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) StopServer() error {
	return s.Shutdown(context.Background())
}
