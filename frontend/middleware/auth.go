package middleware

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gofiber/fiber/v2"
)

var apiEndpoint = os.Getenv("API_ENDPOINT")

// AuthMiddleware validates the JWT token
func AuthMiddleware(c *fiber.Ctx) error {
    log.Printf("AuthMiddleware: Validating token for path: %s", c.Path())

    token := c.Cookies("token")
    if token == "" {
        log.Println("AuthMiddleware: No token cookie found, redirecting to login")
        return c.Redirect("/login")
    }

    // Validate token with backend API
    apiURL := apiEndpoint + "/api/auth/validate"
    log.Printf("AuthMiddleware: Calling validation API at: %s", apiURL)

    req, err := http.NewRequest("POST", apiURL, nil)
    if err != nil {
        log.Printf("AuthMiddleware: Failed to create request: %v", err)
        return c.Redirect("/login")
    }
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("AuthMiddleware: Validation request failed: %v", err)
        return c.Redirect("/login")
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        log.Printf("AuthMiddleware: Validation failed with status: %d", resp.StatusCode)
        return c.Redirect("/login")
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("AuthMiddleware: Failed to read response body: %v", err)
        return c.Redirect("/login")
    }

    log.Printf("AuthMiddleware: Validation response: %s", string(body))

    var result struct {
        Success bool `json:"success"`
        Data    struct {
            Username string `json:"username"`
        } `json:"data"`
    }

    if err := json.Unmarshal(body, &result); err != nil {
        log.Printf("AuthMiddleware: Failed to parse response: %v", err)
        return c.Redirect("/login")
    }

    if !result.Success {
        log.Println("AuthMiddleware: Token validation failed")
        return c.Redirect("/login")
    }

    log.Printf("AuthMiddleware: Token validated successfully for user: %s", result.Data.Username)

    // Store username in context
    c.Locals("username", result.Data.Username)

    return c.Next()
}

// APIAuthMiddleware validates JWT for API requests
func APIAuthMiddleware(c *fiber.Ctx) error {
    log.Printf("APIAuthMiddleware: Validating token for API path: %s", c.Path())

    token := c.Cookies("token")
    if token == "" {
        log.Println("APIAuthMiddleware: No token cookie found")
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    // Validate token with backend API
    apiURL := apiEndpoint + "/api/auth/validate"
    log.Printf("APIAuthMiddleware: Calling validation API at: %s", apiURL)

    req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer([]byte{}))
    if err != nil {
        log.Printf("APIAuthMiddleware: Failed to create request: %v", err)
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("APIAuthMiddleware: Validation request failed: %v", err)
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        log.Printf("APIAuthMiddleware: Validation failed with status: %d", resp.StatusCode)
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("APIAuthMiddleware: Failed to read response body: %v", err)
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    log.Printf("APIAuthMiddleware: Validation response: %s", string(body))

    var result struct {
        Success bool `json:"success"`
        Data    struct {
            Username string `json:"username"`
        } `json:"data"`
    }

    if err := json.Unmarshal(body, &result); err != nil {
        log.Printf("APIAuthMiddleware: Failed to parse response: %v", err)
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    if !result.Success {
        log.Println("APIAuthMiddleware: Token validation failed")
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized",
        })
    }

    log.Printf("APIAuthMiddleware: Token validated successfully for user: %s", result.Data.Username)

    // Store username in context
    c.Locals("username", result.Data.Username)

    return c.Next()
}
