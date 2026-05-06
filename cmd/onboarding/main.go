package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"

	useradapter "todoe/domain/user/adapter"
	userhttp "todoe/domain/user/adapter/http"
	userapplication "todoe/domain/user/application"
	userdomain "todoe/domain/user/domain"

	"todoe/internal/event"
	"todoe/internal/messaging"
)

type multiPublisher struct{ publishers []event.Publisher }

func (m *multiPublisher) Publish(ctx context.Context, e event.Event) {
	for _, p := range m.publishers {
		p.Publish(ctx, e)
	}
}

func main() {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		mysqlDSN = "todoe:todoe@tcp(localhost:3333)/todoe_onboarding?parseTime=true&multiStatements=true"
	}
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	db, err := sqlx.ConnectContext(context.Background(), "mysql", mysqlDSN)
	if err != nil {
		log.Fatal("mysql:", err)
	}
	defer db.Close()

	conn, ch, err := messaging.Connect(amqpURL)
	if err != nil {
		log.Fatal("rabbit:", err)
	}
	defer conn.Close()

	if err := messaging.DeclareTopology(ch, []messaging.Binding{
		{Exchange: messaging.UserExchange, Queue: messaging.QueueWelcomeUserEvents},
		{Exchange: messaging.UserExchange, Queue: messaging.QueueCreditUserEvents},
		{Exchange: messaging.UserExchange, Queue: messaging.QueueAuthenUserEvents},
		{Exchange: messaging.CreditResultExchange, Queue: messaging.QueueOnboardingCreditResults},
	}); err != nil {
		log.Fatal("rabbit topology:", err)
	}

	if err := useradapter.Migrate(db); err != nil {
		log.Fatal("mysql migrate:", err)
	}
	userRepo := useradapter.NewMySQLRepository(db)

	userBus := event.NewEventBus()
	userProjection := useradapter.NewProjectionHandler(userRepo)
	userBus.Subscribe(userdomain.EventRegistered, userProjection)
	userBus.Subscribe(userdomain.EventEmailVerified, userProjection)
	userBus.Subscribe(userdomain.EventCreditScored, userProjection)
	userBus.Subscribe(userdomain.EventProfileCompleted, userProjection)
	userBus.Subscribe(userdomain.EventContactUpdated, userProjection)

	userPublisher := &multiPublisher{publishers: []event.Publisher{
		userBus,
		messaging.NewPublisher(ch, messaging.UserExchange),
	}}

	userService := userapplication.NewService(userRepo, userPublisher)
	userHandler := userhttp.NewHandler(userService)

	// Subscribe to credit scoring results to update user credit status
	if err := messaging.Subscribe(ch, messaging.CreditResultExchange, messaging.QueueOnboardingCreditResults, func(msg messaging.Message) {
		if msg.Type != userdomain.EventCreditScored {
			return
		}
		var p userdomain.CreditScoredPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			slog.Error("onboarding: credit scored unmarshal", "err", err)
			return
		}
		if r := userService.RecordCreditScore(context.Background(), p.UserID, p.Score, p.Approved); r.IsError() {
			slog.Error("onboarding: record credit score", "err", r.Error())
		}
	}); err != nil {
		log.Fatal("rabbit subscribe credit.results:", err)
	}

	// Route credit scoring results to welcome email if approved
	app := fiber.New()
	app.Post("/users/register", userHandler.Register)
	app.Get("/users/activated", userHandler.ListActivated)
	app.Get("/users/:id", userHandler.GetUser)
	app.Patch("/users/:id", userHandler.UpdateContact)
	app.Post("/users/:id/verify-email", userHandler.VerifyEmail)
	app.Post("/users/:id/complete-profile", userHandler.CompleteProfile)

	log.Fatal(app.Listen(":3002"))
}
