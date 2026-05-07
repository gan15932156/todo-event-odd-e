package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	audittAdapter "todoe/internal/audit/adapter"
	"todoe/internal/event"
	healthadapter "todoe/internal/health/adapter"
	healthhttp "todoe/internal/health/adapter/http"
	healthapp "todoe/internal/health/application"
	slaAdapter "todoe/internal/sla/adapter"

	taskadapter "todoe/domain/task/adapter"
	taskhttp "todoe/domain/task/adapter/http"
	taskapplication "todoe/domain/task/application"
	taskdomain "todoe/domain/task/domain"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://root:root@localhost:27017"
	}

	clientIO := mo.NewIOEither(func() (*mongo.Client, error) {
		return mongo.Connect(options.Client().ApplyURI(mongoURI))
	})

	bus := event.NewEventBus()

	healthRepo := healthadapter.NewMongoRepository(clientIO)
	defer healthRepo.Disconnect(context.Background())

	healthService := healthapp.NewService(healthRepo)
	healthHandler := healthhttp.NewHandler(healthService)

	taskRepo := taskadapter.NewMongoRepository(clientIO)
	taskService := taskapplication.NewService(taskRepo, bus)
	taskHandler := taskhttp.NewHandler(taskService)

	auditRepo := audittAdapter.NewMongoRepository(clientIO)
	auditHandler := audittAdapter.NewAuditHandler(auditRepo)
	bus.Subscribe(taskdomain.EventCreated, auditHandler)
	bus.Subscribe(taskdomain.EventStatusChanged, auditHandler)

	slaRepo := slaAdapter.NewCSVRepository()
	slaHandler := slaAdapter.NewHandler(slaRepo)
	bus.Subscribe(taskdomain.EventCreated, slaHandler)
	bus.Subscribe(taskdomain.EventStatusChanged, slaHandler)

	app := fiber.New()

	app.Get("/health", healthHandler.CheckHealth)
	app.Post("/tasks", taskHandler.Create)
	app.Get("/tasks", taskHandler.List)
	app.Get("/tasks/:id", taskHandler.Detail)
	app.Patch("/tasks/:id/status", taskHandler.ChangeStatus)

	log.Fatal(app.Listen(":3000"))
}
