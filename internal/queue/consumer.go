package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
	"github.com/redis/go-redis/v9"
)

const (
	consumerGroup = "pdf-main"
	consumerName  = "pdf-main-1"
)

// HandleResult maps an extraction result to a persistence update. It is a pure
// function (no Redis, no HTTP), so it can be tested in isolation.
//
//   - done:   persists content (Markdown) + status "done".
//   - failed: persists status "failed" + the short error detail taken from the
//     downstream RFC 9457 problem.
func HandleResult(ctx context.Context, result dto.ExtractionResult, p orchestrator.Persistence) error {
	switch result.Status {
	case dto.StatusDone:
		_, err := p.Update(ctx, result.PdfID, dto.PersistUpdateRequest{
			Content: result.Content,
			Status:  dto.StatusDone,
		})
		return err
	case dto.StatusFailed:
		detail := ""
		if result.Error != nil {
			detail = result.Error.Detail
		}
		_, err := p.Update(ctx, result.PdfID, dto.PersistUpdateRequest{
			Status: dto.StatusFailed,
			Error:  &detail,
		})
		return err
	default:
		return fmt.Errorf("unexpected result status %q for pdf %q", result.Status, result.PdfID)
	}
}

// Consumer reads queue:extraction-results with a consumer group and updates
// persistence via HandleResult. It is a thin adapter: no business logic lives
// here, and it is validated manually against pdf-infra (not unit tested).
type Consumer struct {
	rdb         *redis.Client
	persistence orchestrator.Persistence
}

// NewConsumer builds a consumer connected to the Redis queue at addr.
func NewConsumer(addr string, persistence orchestrator.Persistence) *Consumer {
	return &Consumer{
		rdb:         redis.NewClient(&redis.Options{Addr: addr}),
		persistence: persistence,
	}
}

// Run consumes results until ctx is cancelled. It returns nil on a clean
// shutdown (context cancellation).
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamExtractionResults, ">"},
			Count:    10,
			Block:    0, // block until a message arrives or ctx is cancelled
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			slog.Error("reading results stream", "err", err)
			return err
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				if err := c.process(ctx, msg); err != nil {
					// Leave the message unacked so it is redelivered later.
					slog.Error("processing result", "id", msg.ID, "err", err)
					continue
				}
				if err := c.rdb.XAck(ctx, streamExtractionResults, consumerGroup, msg.ID).Err(); err != nil {
					return err
				}
			}
		}
	}
}

// Close releases the underlying Redis connection pool.
func (c *Consumer) Close() error {
	return c.rdb.Close()
}

// ensureGroup creates the consumer group if it does not exist yet.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, streamExtractionResults, consumerGroup, "$").Err()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

// process parses a stream message and applies HandleResult.
func (c *Consumer) process(ctx context.Context, msg redis.XMessage) error {
	result, err := parseResult(msg.Values)
	if err != nil {
		return err
	}
	return HandleResult(ctx, result, c.persistence)
}

// parseResult decodes the individual stream fields into an ExtractionResult.
//
// Field layout of queue:extraction-results (coordination point with
// pdf-extractor): pdf_id, status, content (Markdown, when done) and error (a
// JSON-encoded RFC 9457 problem, when failed).
func parseResult(values map[string]any) (dto.ExtractionResult, error) {
	result := dto.ExtractionResult{
		PdfID:  stringField(values, "pdf_id"),
		Status: dto.Status(stringField(values, "status")),
	}

	if content := stringField(values, "content"); content != "" {
		result.Content = &content
	}

	if raw := stringField(values, "error"); raw != "" {
		var pd problem.ProblemDetails
		if err := json.Unmarshal([]byte(raw), &pd); err != nil {
			return dto.ExtractionResult{}, fmt.Errorf("decoding error field: %w", err)
		}
		result.Error = &pd
	}

	return result, nil
}

func stringField(values map[string]any, key string) string {
	s, _ := values[key].(string)
	return s
}

// isBusyGroup reports whether err is Redis' BUSYGROUP error (the consumer
// group already exists). go-redis does not expose it as a sentinel.
func isBusyGroup(err error) bool {
	return strings.Contains(err.Error(), "BUSYGROUP")
}
