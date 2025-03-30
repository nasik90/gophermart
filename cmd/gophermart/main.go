package main

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nasik90/gophermart/cmd/gophermart/settings"
	"github.com/nasik90/gophermart/internal/app/handler"
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
	s := service.NewService(repo, options.CheckOrderID)
	h := handler.NewHandler(s)
	go s.HandleOrderQueue(options.AccrualServerAddress)
	go s.HandleBadOrdersQueue()

	server := server.NewServer(h, options.ServerAddress)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-sigs
		logger.Log.Info("closing the server")
		if err := server.StopServer(); err != nil {
			logger.Log.Error("stop http server", zap.String("error", err.Error()))
		}
		logger.Log.Info("closing the storage")
		if err := repo.Close(); err != nil {
			logger.Log.Error("close storage", zap.String("error", err.Error()))
		}
		logger.Log.Info("ready to exit")
	}()

	err = server.RunServer()
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("run server", zap.String("error", err.Error()))
		}
	}

	wg.Wait()
	logger.Log.Info("closed gracefuly")
}
