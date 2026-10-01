// Command pdf-main is the business-logic orchestrator of Parse Documents Fast.
// It exposes the public HTTP API and coordinates the downstream services
// (validator, extractor, converter, persistence) plus the Redis extraction
// queue.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/config"
	"github.com/Parse-Documents-Fast/pdf-main/internal/httpapi"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/queue"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	persistence := clients.NewPersistence(cfg.PersistenceURL)
	producer := queue.NewProducer(cfg.RedisQueueAddr)
	consumer := queue.NewConsumer(cfg.RedisQueueAddr, persistence)
	defer producer.Close()
	defer consumer.Close()

	ports := orchestrator.Ports{
		Validator:   clients.NewValidator(cfg.ValidatorURL),
		Persistence: persistence,
		Converter:   clients.NewConverter(cfg.ConverterURL),
		Extractor:   clients.NewExtractor(cfg.ExtractorURL),
		Queue:       producer,
	}

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpapi.Router(ports),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("consumer started", "stream", "queue:extraction-results")
		if err := consumer.Run(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-sigCh:
		slog.Info("shutdown signal received")
	case err := <-errCh:
		slog.Error("component failed", "err", err)
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown", "err", err)
	}

	wg.Wait()
	return nil
}
