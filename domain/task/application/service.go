package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"todoe/domain/task/domain"
	"todoe/domain/task/port"
	"todoe/internal/event"
	SLAdomain "todoe/internal/sla/domain"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	ErrInvalidTitle  = errors.New("title must not be empty")
	ErrInvalidStatus = errors.New("status must be one of: pending, in_progress, done")
)

type Service struct {
	repo port.Repository
	bus  port.Publisher
}

var _ port.UseCase = (*Service)(nil)

func NewService(repo port.Repository, bus port.Publisher) *Service {
	return &Service{repo: repo, bus: bus}
}

func validateTitle(title string) error {
	if title == "" {
		return ErrInvalidTitle
	}
	return nil
}

func validateStatus(s domain.Status) error {
	switch s {
	case domain.StatusPending, domain.StatusInProgress, domain.StatusDone:
		return nil
	}
	return ErrInvalidStatus
}

func (s *Service) CreateTask(ctx context.Context, title string) mo.Result[domain.Task] {
	if err := validateTitle(title); err != nil {
		return mo.Err[domain.Task](err)
	}
	task := domain.NewTask(title)
	if result := s.repo.Save(ctx, task); result.IsError() {
		return mo.Err[domain.Task](result.Error())
	}
	// Publish SLA event for task creation
	sla := SLAdomain.SLA{ID: task.ID.Hex(), Time: time.Now(), Event: "task_created"}
	s.bus.Publish(ctx, event.Event{Type: domain.EventCreated, Payload: task})
	s.bus.Publish(ctx, event.Event{Type: "sla_event", Payload: sla})
	return mo.Ok(task)
}

func (s *Service) ListTasks(ctx context.Context) mo.Result[[]domain.Task] {
	return s.repo.FindAll(ctx)
}

func (s *Service) GetTask(ctx context.Context, id bson.ObjectID) mo.Result[domain.Task] {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) ChangeStatus(ctx context.Context, task domain.Task, status domain.Status) mo.Result[domain.Task] {
	if err := validateStatus(status); err != nil {
		return mo.Err[domain.Task](err)
	}
	next := task.ChangeStatus(status)
	if result := s.repo.Save(ctx, next); result.IsError() {
		return mo.Err[domain.Task](result.Error())
	}
	// Publish SLA event for status change
	s.bus.Publish(ctx, event.Event{Type: domain.EventStatusChanged, Payload: next})
	if status == domain.StatusDone {
		duration := time.Since(task.CreatedAt)
		sla := SLAdomain.SLA{ID: task.ID.Hex(), Time: time.Now(), Event: fmt.Sprintf("task_done_duration:%s", duration)}
		s.bus.Publish(ctx, event.Event{Type: "sla_event", Payload: sla})
	}
	return mo.Ok(next)
}
