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

// Definimos las operaciones necesarias para controlar
// el inicio y apagado del servidor.
type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

// Configuramos las dependencias y los tiempos de espera del servidor.
func newServer() *http.Server {
	memoryStore := store.NewMemoryStore()
	walletService := service.NewWalletService(memoryStore)
	walletHandler := handler.NewHandler(walletService)

	return &http.Server{
		Addr:              ":8080",
		Handler:           walletHandler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Ejecutamos el servidor y controlamos su ciclo de vida.
func runServer(
	ctx context.Context,
	server httpServer,
	logger *slog.Logger,
) error {
	serverErrors := make(chan error, 1)

	go func() {
		logger.Info(
			"iniciando servidor",
			"port", 8080,
			"service", "wallet-service",
		)

		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(
				"error al iniciar el servidor",
				"error", err,
			)

			return err
		}

		return nil

	case <-ctx.Done():
		logger.Info("iniciando apagado del servidor")

		// Permitimos finalizar las solicitudes que estén en curso.
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error(
				"error durante el apagado del servidor",
				"error", err,
			)

			_ = server.Close()

			return err
		}

		logger.Info("servidor detenido correctamente")

		return nil
	}
}

// Inicializamos la aplicación y retornamos el código de salida.
func run(
	ctx context.Context,
	server httpServer,
	logger *slog.Logger,
) int {
	if err := runServer(ctx, server, logger); err != nil {
		return 1
	}

	return 0
}

// Configuramos los logs y las señales de cierre del sistema.
func start() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	return run(ctx, newServer(), logger)
}

// Permite controlar la salida del proceso durante las pruebas.
var exitProcess = os.Exit

func main() {
	exitProcess(start())
}
