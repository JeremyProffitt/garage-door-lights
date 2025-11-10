package main

import (
    "context"
    "embed"
    "log"
    "net/http"
    "os"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2/middleware/recover"
    "github.com/gofiber/fiber/v2/middleware/filesystem"
    fiberadapter "github.com/awslabs/aws-lambda-go-api-proxy/fiber"
    "github.com/gofiber/template/html/v2"

    "candle-lights/frontend/handlers"
    "candle-lights/frontend/middleware"
)

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var staticFiles embed.FS

var fiberLambda *fiberadapter.FiberLambda

func init() {
    // Create template engine
    engine := html.NewFileSystem(http.FS(templates), ".html")

    // Create Fiber app
    app := fiber.New(fiber.Config{
        Views: engine,
    })

    // Middleware
    app.Use(recover.New())
    app.Use(logger.New())

    // HTTPS redirect middleware
    app.Use(func(c *fiber.Ctx) error {
        // Check X-Forwarded-Proto header (set by API Gateway/Load Balancer)
        proto := c.Get("X-Forwarded-Proto", "https")

        // If request came via HTTP, redirect to HTTPS
        if proto == "http" {
            host := c.Hostname()
            path := c.OriginalURL()
            return c.Redirect("https://"+host+path, 301)
        }
        return c.Next()
    })

    // Static files
    app.Use("/static", filesystem.New(filesystem.Config{
        Root: http.FS(staticFiles),
    }))

    // Routes
    setupRoutes(app)

    // Create Lambda adapter
    fiberLambda = fiberadapter.New(app)
}

func setupRoutes(app *fiber.App) {
    // Public routes
    app.Get("/", handlers.IndexHandler)
    app.Get("/login", handlers.LoginPageHandler)
    app.Get("/register", handlers.RegisterPageHandler)

    // Protected routes (require auth)
    app.Get("/dashboard", middleware.AuthMiddleware, handlers.DashboardHandler)
    app.Get("/patterns", middleware.AuthMiddleware, handlers.PatternsHandler)
    app.Get("/devices", middleware.AuthMiddleware, handlers.DevicesHandler)
    app.Get("/settings", middleware.AuthMiddleware, handlers.SettingsHandler)

    // API proxy routes for AJAX calls
    api := app.Group("/api")
    api.Use(middleware.APIAuthMiddleware)
    api.Get("/patterns", handlers.GetPatternsHandler)
    api.Post("/patterns", handlers.CreatePatternHandler)
    api.Put("/patterns/:id", handlers.UpdatePatternHandler)
    api.Delete("/patterns/:id", handlers.DeletePatternHandler)
    api.Get("/devices", handlers.GetDevicesHandler)
    api.Post("/devices", handlers.CreateDeviceHandler)
    api.Put("/devices/:id/pattern", handlers.AssignPatternHandler)
    api.Post("/particle/command", handlers.SendCommandHandler)

    // Auth routes
    app.Post("/auth/login", handlers.LoginHandler)
    app.Post("/auth/register", handlers.RegisterHandler)
    app.Get("/auth/logout", handlers.LogoutHandler)
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    return fiberLambda.ProxyWithContext(ctx, req)
}

func main() {
    if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
        // Running in Lambda
        lambda.Start(handler)
    } else {
        // Running locally
        app := fiber.New()
        setupRoutes(app)
        log.Fatal(app.Listen(":3000"))
    }
}
