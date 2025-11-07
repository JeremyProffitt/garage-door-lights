package middleware

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "os"

    "github.com/gofiber/fiber/v2"
)

var apiEndpoint = os.Getenv("API_ENDPOINT")

// AuthMiddleware validates the JWT token
func AuthMiddleware(c *fiber.Ctx) error {
    token := c.Cookies("token")
    if token == "" {
        return c.Redirect("/login")
    }

    // Validate token with backend
    req, _ := http.NewRequest("POST", apiEndpoint+"/auth/validate", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil || resp.StatusCode != 200 {
        return c.Redirect("/login")
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var result struct {
        Success bool `json:"success"`
        Data    struct {
            Username string `json:"username"`
        } `json:"data"`
    }

    json.Unmarshal(body, &result)

    if !result.Success {
        return c.Redirect("/login")
    }

    // Store username in context
    c.Locals("username", result.Data.Username)

    return c.Next()
}

// APIAuthMiddleware validates JWT for API requests
func APIAuthMiddleware(c *fiber.Ctx) error {
    token := c.Cookies("token")
    if token == "" {
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    // Validate token with backend
    req, _ := http.NewRequest("POST", apiEndpoint+"/auth/validate", bytes.NewBuffer([]byte{}))
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil || resp.StatusCode != 200 {
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var result struct {
        Success bool `json:"success"`
        Data    struct {
            Username string `json:"username"`
        } `json:"data"`
    }

    json.Unmarshal(body, &result)

    if !result.Success {
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    // Store username in context
    c.Locals("username", result.Data.Username)

    return c.Next()
}
