package adapter

import (
	"context"
	"fmt"
	"log/slog"

	"todoe/domain/user/domain"
	"todoe/internal/event"
)

func NewProjectionHandler(repo *MySQLRepository) func(context.Context, event.Event) error {
	return func(ctx context.Context, e event.Event) error {
		user, ok := e.Payload.(domain.User)
		if !ok {
			return fmt.Errorf("unexpected payload type %T", e.Payload)
		}
		slog.Info(user.Email)
		result := repo.Upsert(ctx, user)
		if result.IsError() {
			return result.Error()
		}
		return nil
	}
}
