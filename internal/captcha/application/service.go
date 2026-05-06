package application

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/bson"

	"todoe/internal/captcha/domain"
	"todoe/internal/captcha/port"
	"todoe/internal/event"
	"todoe/internal/messaging"
)

const challengeTTL = 5 * time.Minute

type Service struct {
	repo      port.Repository
	publisher port.Publisher
	messaging *messaging.Publisher
}

var _ port.UseCase = (*Service)(nil)

func NewService(repo port.Repository, publisher port.Publisher, messaging *messaging.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher, messaging: messaging}
}

func (s *Service) Issue(ctx context.Context) mo.Result[domain.Challenge] {
	a := rand.IntN(9) + 1
	b := rand.IntN(9) + 1
	now := time.Now()
	challenge := domain.Challenge{
		ID:        bson.NewObjectID(),
		Question:  fmt.Sprintf("%d + %d", a, b),
		Answer:    a + b,
		Verified:  false,
		IssuedAt:  now,
		ExpiresAt: now.Add(challengeTTL),
	}
	payload := domain.IssuedPayload{
		ID:        challenge.ID,
		Question:  challenge.Question,
		Answer:    challenge.Answer,
		ExpiresAt: challenge.ExpiresAt,
	}
	if r := s.repo.Append(ctx, challenge.ID, domain.EventIssued, payload); r.IsError() {
		return mo.Err[domain.Challenge](r.Error())
	}
	s.publisher.Publish(ctx, event.Event{Type: domain.EventIssued, Payload: challenge})
	return mo.Ok(challenge)
}

func (s *Service) Verify(ctx context.Context, id bson.ObjectID, answer int) mo.Result[domain.Challenge] {
	found := s.repo.FindChallenge(ctx, id)
	if found.IsError() {
		return mo.Err[domain.Challenge](ErrChallengeNotFound)
	}
	challenge := found.MustGet()

	if challenge.IsExpired() {
		failed := domain.FailedPayload{ID: id, Reason: "expired"}
		s.repo.Append(ctx, id, domain.EventFailed, failed)
		s.publisher.Publish(ctx, event.Event{Type: domain.EventFailed, Payload: failed})
		return mo.Err[domain.Challenge](ErrChallengeExpired)
	}
	if challenge.Verified {

		return mo.Err[domain.Challenge](ErrChallengeAlreadyUsed)
	}
	if answer != challenge.Answer {
		failed := domain.FailedPayload{ID: id, Reason: "incorrect"}
		s.repo.Append(ctx, id, domain.EventFailed, failed)
		s.publisher.Publish(ctx, event.Event{Type: domain.EventFailed, Payload: failed})
		return mo.Err[domain.Challenge](ErrIncorrectAnswer)
	}

	verified := challenge
	verified.Verified = true
	if r := s.repo.Append(ctx, id, domain.EventVerified, domain.VerifiedPayload{ID: id}); r.IsError() {
		return mo.Err[domain.Challenge](r.Error())
	}

	s.publisher.Publish(ctx, event.Event{Type: domain.EventVerified, Payload: verified})
	s.messaging.Publish(ctx, event.Event{Type: domain.EventVerified, Payload: verified})
	return mo.Ok(verified)
}
