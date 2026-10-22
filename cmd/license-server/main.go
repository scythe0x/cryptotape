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

	"cryptotape/internal/licenseserver"
)

func run() error {
	c, err := licenseserver.LoadConfig()
	if err != nil {
		return err
	}
	store, err := licenseserver.Open(c.Database, c.Encryption, c.Lookup)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	bot := &licenseserver.Bot{Token: c.BotToken, Store: store}
	if c.BotToken == "" {
		return errors.New("MYTHRA_BOT_TOKEN is required")
	}
	botDone := make(chan error, 1)
	go func() { botDone <- bot.Run(ctx) }()
	server := &http.Server{Addr: "127.0.0.1:9087", Handler: licenseserver.Handler(store, c.Signing), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.ListenAndServe() }()
	botFinished := false
	select {
	case <-ctx.Done():
	case err = <-serverDone:
		cancel()
	case err = <-botDone:
		botFinished = true
		cancel()
	}
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if shutdownErr := server.Shutdown(shutdown); shutdownErr != nil {
		return shutdownErr
	}
	if !botFinished {
		select {
		case botErr := <-botDone:
			err = errors.Join(err, botErr)
		case <-shutdown.Done():
			return errors.Join(err, shutdown.Err())
		}
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	if err := run(); err != nil {
		slog.Error("license service stopped", "error", err)
		os.Exit(1)
	}
}
