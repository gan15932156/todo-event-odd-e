package domain

import (
	"encoding/json"
	"time"
)

type OnboardingStatus string

const (
	StatusRegistered         OnboardingStatus = "registered"
	StatusEmailVerified      OnboardingStatus = "email_verified"
	StatusCreditApproved     OnboardingStatus = "credit_approved"
	StatusCreditDenied       OnboardingStatus = "credit_denied"
	StatusOnboardingComplete OnboardingStatus = "onboarding_complete"
)

const (
	EventRegistered       = "user.registered"
	EventEmailVerified    = "user.email_verified"
	EventCreditScored     = "user.credit_scored"
	EventProfileCompleted = "user.profile_completed"
	EventProfileUpdated   = "user.profile_updated"
	EventUserActivated    = "user.activated"
	EventContactUpdated   = "user.contact_updated"
)

type User struct {
	ID                string           `db:"id"                 json:"id"`
	Name              string           `db:"name"               json:"name"`
	Email             string           `db:"email"              json:"email"`
	Bio               string           `db:"bio"                json:"bio"`
	Status            OnboardingStatus `db:"status"             json:"status"`
	VerificationToken string           `db:"verification_token" json:"verification_token"`
	CreditScore       int              `db:"credit_score"       json:"credit_score"`
	CreditApproved    bool             `db:"credit_approved"    json:"credit_approved"`
	CreatedAt         time.Time        `db:"created_at"         json:"created_at"`
}

type RegisteredPayload struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	VerificationToken string `json:"verification_token"`
}

type EmailVerifiedPayload struct {
	UserID string `json:"user_id"`
}

type CreditScoredPayload struct {
	UserID   string `json:"user_id"`
	Score    int    `json:"score"`
	Approved bool   `json:"approved"`
}

type ProfileCompletedPayload struct {
	UserID string `json:"user_id"`
	Bio    string `json:"bio"`
}

type UserActivatedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type ContactUpdatedPayload struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Bio    string `json:"bio"`
}

type UserEvent struct {
	ID          string          `db:"id"           json:"id"`
	AggregateID string          `db:"aggregate_id" json:"aggregate_id"`
	Type        string          `db:"type"         json:"type"`
	Payload     json.RawMessage `db:"payload"      json:"payload"`
	CreatedAt   time.Time       `db:"created_at"   json:"created_at"`
}

func (u User) WithEmailVerified() User {
	u.Status = StatusEmailVerified
	return u
}

func (u User) WithCreditScore(score int, approved bool) User {
	u.CreditScore = score
	u.CreditApproved = approved
	if approved {
		u.Status = StatusCreditApproved
	} else {
		u.Status = StatusCreditDenied
	}
	return u
}

func (u User) WithProfile(bio string) User {
	u.Bio = bio
	u.Status = StatusOnboardingComplete
	return u
}

func (u User) WithContact(name, email, bio string) User {
	u.Name = name
	u.Email = email
	u.Bio = bio
	return u
}
