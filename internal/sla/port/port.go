package port

import (
	"context"
	"todoe/internal/sla/domain"

	"github.com/samber/mo"
)

// this port defined for create csv file with header sla-2026-05-07.csv
// this file contain time and status of task
type Repository interface {
	WriteCSV(ctx context.Context, sla domain.SLA) mo.Result[struct{}]
	// Save(ctx context.Context, task domain.) mo.Result[struct{}]
	// FindAll(ctx context.Context) mo.Result[[]domain.Task]
	// FindByID(ctx context.Context, id bson.ObjectID) mo.Result[domain.Task]
}
