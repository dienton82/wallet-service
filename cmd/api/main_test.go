package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Simulamos el comportamiento del servidor para probar
// los diferentes escenarios sin ocupar el puerto 8080.
type mockServer struct {
	listenErr   error
	shutdownErr error
	started     chan struct{}
	stopped     chan struct{}
	closed      bool
}

func (m *mockServer) ListenAndServe() error {
	if m.started != nil {
		close(m.started)
	}

	if m.stopped != nil {
		<-m.stopped
	}

	return m.listenErr
}

func (m *mockServer) Shutdown(ctx context.Context) error {
	if m.stopped != nil {
		close(m.stopped)
	}

	return m.shutdownErr
}

func (m *mockServer) Close() error {
	m.closed = true

	return nil
}

// Creamos un logger para las pruebas.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// Validamos la configuración inicial del servidor.
func TestNewServer(t *testing.T) {
	server := newServer()

	if server.Addr != ":8080" {
		t.Errorf("puerto incorrecto: %s", server.Addr)
	}

	if server.Handler == nil {
		t.Fatal("el servidor debe tener un handler")
	}

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Error("tiempo de lectura de encabezados incorrecto")
	}

	if server.ReadTimeout != 10*time.Second {
		t.Error("tiempo de lectura incorrecto")
	}

	if server.WriteTimeout != 10*time.Second {
		t.Error("tiempo de escritura incorrecto")
	}

	if server.IdleTimeout != 60*time.Second {
		t.Error("tiempo de espera incorrecto")
	}

	// Verificamos que las dependencias estén conectadas.
	request := httptest.NewRequest(
		http.MethodPost,
		"/accounts",
		strings.NewReader(
			`{"owner":"Ana","initial_balance":10000}`,
		),
	)

	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Errorf(
			"código esperado 201, obtenido %d",
			response.Code,
		)
	}
}

// Validamos el cierre normal del servidor.
func TestRunServerClosed(t *testing.T) {
	server := &mockServer{
		listenErr: http.ErrServerClosed,
	}

	err := runServer(
		context.Background(),
		server,
		newTestLogger(),
	)

	if err != nil {
		t.Errorf("error inesperado: %v", err)
	}
}

// Validamos un error durante el inicio del servidor.
func TestRunServerListenError(t *testing.T) {
	expected := errors.New("puerto ocupado")

	server := &mockServer{
		listenErr: expected,
	}

	err := runServer(
		context.Background(),
		server,
		newTestLogger(),
	)

	if !errors.Is(err, expected) {
		t.Errorf(
			"error esperado %v, obtenido %v",
			expected,
			err,
		)
	}
}

// Validamos el apagado ordenado del servidor.
func TestRunServerGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := &mockServer{
		started:   make(chan struct{}),
		stopped:   make(chan struct{}),
		listenErr: http.ErrServerClosed,
	}

	var logs bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	result := make(chan error, 1)

	go func() {
		result <- runServer(ctx, server, logger)
	}()

	select {
	case <-server.started:
	case <-time.After(5 * time.Second):
		t.Fatal("el servidor no inició")
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Errorf("error durante el apagado: %v", err)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("el servidor no se detuvo")
	}

	if server.closed {
		t.Error("no se esperaba un cierre forzado")
	}

	if !strings.Contains(
		logs.String(),
		"servidor detenido correctamente",
	) {
		t.Error("no se encontró el log de apagado")
	}
}

// Validamos el cierre forzado cuando falla el apagado ordenado.
func TestRunServerShutdownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expected := errors.New("error de apagado")

	server := &mockServer{
		started:     make(chan struct{}),
		stopped:     make(chan struct{}),
		listenErr:   http.ErrServerClosed,
		shutdownErr: expected,
	}

	result := make(chan error, 1)

	go func() {
		result <- runServer(
			ctx,
			server,
			newTestLogger(),
		)
	}()

	select {
	case <-server.started:
	case <-time.After(5 * time.Second):
		t.Fatal("el servidor no inició")
	}

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, expected) {
			t.Errorf("error inesperado: %v", err)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("el servidor no se detuvo")
	}

	if !server.closed {
		t.Error("se esperaba un cierre forzado")
	}
}

// Validamos los códigos de salida de la aplicación.
func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			"cierre correcto",
			http.ErrServerClosed,
			0,
		},
		{
			"error del servidor",
			errors.New("error de inicio"),
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &mockServer{
				listenErr: tt.err,
			}

			code := run(
				context.Background(),
				server,
				newTestLogger(),
			)

			if code != tt.expected {
				t.Errorf(
					"código esperado %d, obtenido %d",
					tt.expected,
					code,
				)
			}
		})
	}
}

// Validamos el punto de entrada de la aplicación y su código
// de salida sin finalizar el proceso de pruebas.
func TestMainProcess(t *testing.T) {
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		t.Fatalf("no se pudo reservar el puerto 8080: %v", err)
	}

	defer listener.Close()

	// Restauramos la función de salida y el logger al finalizar.
	previousExit := exitProcess
	previousLogger := slog.Default()

	defer func() {
		exitProcess = previousExit
		slog.SetDefault(previousLogger)
	}()

	exitCode := -1

	// Capturamos el código de salida sin cerrar el proceso.
	exitProcess = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Errorf(
			"código esperado 1, obtenido %d",
			exitCode,
		)
	}
}

// Validamos la inicialización de la aplicación y su código
// de salida cuando el puerto se encuentra ocupado.
func TestStart(t *testing.T) {
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		t.Fatalf("no se pudo reservar el puerto 8080: %v", err)
	}

	defer listener.Close()

	// Restauramos el logger después de finalizar la prueba.
	previousLogger := slog.Default()
	defer slog.SetDefault(previousLogger)

	code := start()

	if code != 1 {
		t.Errorf("código esperado 1, obtenido %d", code)
	}
}
