package adapter

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"todoe/internal/audit/domain"
	"todoe/internal/event"
)

func NewAuditHandler(repo *MongoRepository) func(context.Context, event.Event) error {
	return func(ctx context.Context, e event.Event) error {
		entry := domain.AuditEntry{
			ID:        bson.NewObjectID(),
			EventType: string(e.Type),
			Payload:   e.Payload,
			CreatedAt: time.Now(),
		}
		slog.Info("audit: handling event", "event_type", e.Type)
		result := repo.Save(ctx, entry)
		if result.IsError() {
			return result.Error()
		}
		return nil
	}
}
