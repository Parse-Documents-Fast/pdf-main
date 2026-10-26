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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/clients"
	"github.com/Parse-Documents-Fast/pdf-main/internal/config"
	"github.com/Parse-Documents-Fast/pdf-main/internal/httpapi"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/queue"
	redislogging "github.com/redis/go-redis/v9/logging"
)

func main() {
	logger, level := newLogger()
	slog.SetDefault(logger)

	if err := run(logger, level); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// newLogger builds the single structured logger of the service (ADR-0007): a
// JSON handler to stdout with the level from LOG_LEVEL and timestamps in UTC.
// It returns the logger plus the resolved level (lowercase) for the "service
// started" event. An invalid LOG_LEVEL falls back to info with a warning.
func newLogger() (*slog.Logger, string) {
	level := new(slog.LevelVar)
	raw := os.Getenv("LOG_LEVEL")
	invalid := raw != "" && level.UnmarshalText([]byte(raw)) != nil
	if raw == "" || invalid {
		level.Set(slog.LevelInfo)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				a.Value = slog.TimeValue(a.Value.Time().UTC())
			}
			return a
		},
	})
	logger := slog.New(handler).With("service", "pdf-main")

	if invalid {
		logger.Warn("invalid log level; using info", "value", raw)
	}

	return logger, strings.ToLower(level.Level().String())
}

func run(logger *slog.Logger, level string) error {
	cfg := config.Load()

	// go-redis writes its own stderr lines; disable them so every log line is
	// our structured JSON (ADR-0007). Redis errors are logged by the queue
	// adapters instead.
	redislogging.Disable()

	persistence := clients.NewPersistence(cfg.PersistenceURL)
	producer := queue.NewProducer(cfg.RedisQueueAddr, cfg.StreamExtraction, logger)
	consumer := queue.NewConsumer(cfg.RedisQueueAddr, cfg.StreamExtractionResults, cfg.ConsumerGroup, cfg.ConsumerName, persistence, logger)
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
		Handler: httpapi.Router(logger, ports, cfg.MaxFileSizeMB),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	logger.Info("service started", "http_addr", cfg.HTTPAddr, "log_level", level)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumer.Run(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-sigCh:
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("component failed", "error", err)
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
	}

	wg.Wait()

	logger.Info("service stopped")
	return nil
}
