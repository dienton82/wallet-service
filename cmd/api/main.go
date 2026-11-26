package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dienton82/wallet-service/internal/handler"
	"github.com/dienton82/wallet-service/internal/service"
	"github.com/dienton82/wallet-service/internal/store"
)

func main() {
	// Configuramos los logs en formato JSON.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Inicializamos las capas del servicio.
	memoryStore := store.NewMemoryStore()
	walletService := service.NewWalletService(memoryStore)
	walletHandler := handler.NewHandler(walletService)

	// Configuramos el servidor en el puerto solicitado.
	server := &http.Server{
		Addr:              ":8080",
		Handler:           walletHandler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Escuchamos las señales de cierre del sistema.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	// Iniciamos el servidor sin bloquear el proceso principal.
	go func() {
		slog.Info(
			"servidor iniciado",
			"port", 8080,
			"service", "wallet-service",
		)

		serverErrors <- server.ListenAndServe()
	}()

	// Esperamos una señal de cierre o un error del servidor.
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error(
				"error al iniciar el servidor",
				"error", err,
			)
			os.Exit(1)
		}

	case <-ctx.Done():
		slog.Info("iniciando apagado del servidor")

		// Damos tiempo para finalizar las solicitudes en curso.
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error(
				"error durante el apagado del servidor",
				"error", err,
			)

			_ = server.Close()
			os.Exit(1)
		}

		slog.Info("servidor detenido correctamente")
	}
}
