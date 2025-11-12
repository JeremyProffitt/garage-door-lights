package main

import (
    "context"
    "embed"
    "io/fs"
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

//go:embed static
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
    staticFS, err := fs.Sub(staticFiles, "static")
    if err != nil {
        panic(err)
    }
    app.Use("/static", filesystem.New(filesystem.Config{
        Root: http.FS(staticFS),
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

    // Auth routes (form submissions)
    app.Post("/auth/login", handlers.LoginHandler)
    app.Post("/auth/register", handlers.RegisterHandler)
    app.Get("/auth/logout", handlers.LogoutHandler)

    // API routes (used by JavaScript - proxy to backend)
    app.Post("/api/auth/login", handlers.LoginHandler)
    app.Post("/api/auth/register", handlers.RegisterHandler)

    // API routes for patterns (protected)
    app.Get("/api/patterns", middleware.APIAuthMiddleware, handlers.GetPatternsHandler)
    app.Post("/api/patterns", middleware.APIAuthMiddleware, handlers.CreatePatternHandler)
    app.Put("/api/patterns/:id", middleware.APIAuthMiddleware, handlers.UpdatePatternHandler)
    app.Delete("/api/patterns/:id", middleware.APIAuthMiddleware, handlers.DeletePatternHandler)

    // API routes for devices (protected)
    app.Get("/api/devices", middleware.APIAuthMiddleware, handlers.GetDevicesHandler)
    app.Post("/api/devices", middleware.APIAuthMiddleware, handlers.CreateDeviceHandler)
    app.Put("/api/devices/:id/pattern", middleware.APIAuthMiddleware, handlers.AssignPatternHandler)

    // API routes for particle commands (protected)
    app.Post("/api/particle/command", middleware.APIAuthMiddleware, handlers.SendCommandHandler)
    app.Post("/api/particle/devices/refresh", middleware.APIAuthMiddleware, handlers.RefreshDevicesHandler)
    app.Post("/api/particle/validate-token", middleware.APIAuthMiddleware, handlers.ValidateParticleTokenHandler)
    app.Post("/api/particle/oauth/initiate", middleware.APIAuthMiddleware, handlers.ParticleOAuthInitiateHandler)

    // API routes for settings (protected)
    app.Post("/api/settings/particle", middleware.APIAuthMiddleware, handlers.UpdateParticleSettingsHandler)
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Printf("=== Frontend Handler Called ===")
    log.Printf("Path: %s", req.Path)
    log.Printf("Method: %s", req.HTTPMethod)
    log.Printf("Source IP: %s", req.RequestContext.Identity.SourceIP)
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
