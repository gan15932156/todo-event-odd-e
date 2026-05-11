package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	healthadapter "todoe/internal/health/adapter"
	healthhttp "todoe/internal/health/adapter/http"
	healthapp "todoe/internal/health/application"

	authenAdapter "todoe/internal/authen/adapter"
	authenhttp "todoe/internal/authen/adapter/http"
	authenapp "todoe/internal/authen/application"
	authendomain "todoe/internal/authen/domain"

	taskadapter "todoe/domain/task/adapter"
	taskhttp "todoe/domain/task/adapter/http"
	taskapplication "todoe/domain/task/application"
	taskdomain "todoe/domain/task/domain"

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
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://root:root@localhost:27017"
	}
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	clientIO := mo.NewIOEither(func() (*mongo.Client, error) {
		return mongo.Connect(options.Client().ApplyURI(mongoURI))
	})

	healthRepo := healthadapter.NewMongoRepository(clientIO)
	defer healthRepo.Disconnect(context.Background())
	healthService := healthapp.NewService(healthRepo)
	healthHandler := healthhttp.NewHandler(healthService)

	conn, ch, err := messaging.Connect(amqpURL)
	if err != nil {
		log.Fatal("rabbit:", err)
	}
	defer conn.Close()

	if err := messaging.DeclareTopology(ch, []messaging.Binding{
		{Exchange: messaging.TaskExchange, Queue: messaging.QueueAuditTaskEvents},
	}); err != nil {
		log.Fatal("rabbit topology:", err)
	}

	// ── Task domain ──────────────────────────────────────────────────────
	taskBus := event.NewEventBus()
	taskRepo := taskadapter.NewMongoRepository(clientIO)
	taskProjection := taskadapter.NewProjectionHandler(taskRepo)
	taskBus.Subscribe(taskdomain.EventCreated, taskProjection)
	taskBus.Subscribe(taskdomain.EventStatusChanged, taskProjection)
	taskPublisher := &multiPublisher{publishers: []event.Publisher{
		taskBus,
		messaging.NewPublisher(ch, messaging.TaskExchange),
	}}
	taskService := taskapplication.NewService(taskRepo, taskPublisher)
	taskHandler := taskhttp.NewHandler(taskService)

	// ── Authen domain ────────────────────────────────────────────────────
	authenBus := event.NewEventBus()
	authenRepo := authenAdapter.NewMongoRepository(clientIO)
	authenProjection := authenAdapter.NewProjectionHandler(authenRepo)
	authenBus.Subscribe(authendomain.EventLoggedIn, authenProjection)
	authenBus.Subscribe(authendomain.EventLoggedOut, authenProjection)
	authenService := authenapp.NewService(authenRepo, authenBus)
	authenHandler := authenhttp.NewHandler(authenService)

	if err := messaging.Subscribe(ch, messaging.UserExchange, messaging.QueueAuthenUserEvents, func(msg messaging.Message) {
		if msg.Type != userdomain.EventContactUpdated {
			return
		}
		var p userdomain.User
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			slog.Error("api: user.activated unmarshal", "err", err)
			return
		}

		if r := authenService.UpdateCredential(context.Background(), p.Email, p.ID); r.IsError() {
			slog.Error("api: user.update credential failed", "err", r.Error())
		}
	}); err != nil {
		log.Fatal("rabbit subscribe authen.user.events:", err)
	}

	// user.activated arrives from cmd/onboarding via RabbitMQ → create auth credential
	if err := messaging.Subscribe(ch, messaging.UserExchange, messaging.QueueAuthenUserEvents, func(msg messaging.Message) {
		if msg.Type != userdomain.EventUserActivated {
			return
		}
		var p userdomain.UserActivatedPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			slog.Error("api: user.activated unmarshal", "err", err)
			return
		}
		if r := authenService.ActivateUser(context.Background(), p.UserID, p.Email, p.Name); r.IsError() {
			slog.Error("api: user.activated credential creation failed", "err", r.Error())
		}
	}); err != nil {
		log.Fatal("rabbit subscribe authen.user.events:", err)
	}

	// ── HTTP ─────────────────────────────────────────────────────────────
	authMiddleware := func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}
		if r := authenService.ValidateToken(c.Context(), token); r.IsError() {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	}

	app := fiber.New()
	app.Get("/health", healthHandler.CheckHealth)

	app.Post("/auth/register", authenHandler.RegisterCredential)
	app.Post("/auth/login", authenHandler.Login)
	app.Post("/auth/logout", authenHandler.Logout)

	tasks := app.Group("/tasks", authMiddleware)
	tasks.Post("/", taskHandler.Create)
	tasks.Get("/", taskHandler.List)
	tasks.Get("/:id", taskHandler.Detail)
	tasks.Patch("/:id/status", taskHandler.ChangeStatus)

	log.Fatal(app.Listen(":3000"))
}
