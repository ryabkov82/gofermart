package main

import (
	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/config"
	"github.com/ryabkov82/gofermart/internal/app/server"
	"github.com/ryabkov82/gofermart/internal/app/utils/logger"
)

func main() {

	cfg := config.Load()

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		panic(err)
	}

	// Запуск сервера с использованием конфигурации
	logger.Log.Info("Starting server", zap.String("address", cfg.HTTPServerAddr))
	//log.Printf("Starting server on %s with base URL %s", cfg.HTTPServerAddr, cfg.BaseURL)
	server.StartServer(logger.Log, cfg)

}
