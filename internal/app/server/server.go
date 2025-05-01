package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/ryabkov82/gofermart/internal/app/config"
	"github.com/ryabkov82/gofermart/internal/app/handlers/users/login"
	"github.com/ryabkov82/gofermart/internal/app/handlers/users/register"
	mwlogger "github.com/ryabkov82/gofermart/internal/app/server/middleware/logger"
	"github.com/ryabkov82/gofermart/internal/app/server/middleware/mwgzip"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/storage/postgres"

	"github.com/go-chi/chi/v5"
)

func StartServer(log *zap.Logger, cfg *config.Config) {

	pg, err := postgres.NewPostgresStorage(cfg.DBConnect)

	if err != nil {
		panic(err)
	}
	srv := service.NewService(pg)

	router := chi.NewRouter()

	router.Use(mwlogger.RequestLogging(log))
	router.Use(mwgzip.Gzip)

	router.Post("/api/user/register", register.GetHandler(srv, []byte(cfg.JwtKey), log))
	router.Post("/api/user/login", login.GetHandler(srv, []byte(cfg.JwtKey), log))

	log.Info("Server started", zap.String("address", cfg.HTTPServerAddr))

	// Запуск HTTP-сервера в отдельной горутине

	server := &http.Server{
		Addr:    cfg.HTTPServerAddr,
		Handler: router, // Ваш роутер
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("failed to serve server", zap.Error(err))
		}
	}()

	// Обработка сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Создаем контекст с таймаутом для graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Остановка HTTP-сервера
	if err := server.Shutdown(ctx); err != nil {
		log.Info("HTTP server shutdown error", zap.Error(err))
	}

	log.Info("Server shutdown complete")

}
