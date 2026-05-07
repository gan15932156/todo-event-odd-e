package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"todoe/internal/event"
	"todoe/internal/sla/domain"
)

// this handler listen event from task and create csv file
type Handler struct {
	repository *CSVRepository
}

func NewHandler(repository *CSVRepository) func(context.Context, event.Event) error {
	return func(ctx context.Context, e event.Event) error {
		slog.Info("sla", e.Payload)
		sla, ok := e.Payload.(domain.SLA)
		if !ok {
			return fmt.Errorf("invalid payload type: expected domain.Task, got %T", e.Payload)
		}

		if result := repository.WriteCSV(ctx, sla); result.IsError() {
			return result.Error()
		}
		return nil
	}
}
