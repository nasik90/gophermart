package main

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nasik90/gophermart/cmd/gophermart/settings"
	handler "github.com/nasik90/gophermart/internal/app/handlers"
	"github.com/nasik90/gophermart/internal/app/logger"
	"github.com/nasik90/gophermart/internal/app/server"
	"github.com/nasik90/gophermart/internal/app/service"
	"github.com/nasik90/gophermart/internal/app/storage/pg"
	"go.uber.org/zap"
)

func main() {
	options := new(settings.Options)
	settings.ParseFlags(options)

	if err := logger.Initialize(options.LogLevel); err != nil {
		panic(err)
	}

	conn, err := sql.Open("pgx", options.DatabaseURI)
	if err != nil {
		logger.Log.Fatal("open pgx conn", zap.String("DatabaseDSN", options.DatabaseURI), zap.String("error", err.Error()))
	}
	repo, err := pg.NewStore(conn)
	if err != nil {
		logger.Log.Fatal("create pg repo", zap.String("DatabaseDSN", options.DatabaseURI), zap.String("error", err.Error()))
	}
	s := service.NewService(repo)
	h := handler.NewHandler(s)
	serv := server.NewServer(h, options.ServerAddress)
	serv.RunServer()
}
