package handlers

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "time"

    "github.com/gofiber/fiber/v2"
)

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type RegisterRequest struct {
    Username         string `json:"username"`
    Password         string `json:"password"`
    Email            string `json:"email"`
    ParticleUsername string `json:"particleUsername"`
    ParticleToken    string `json:"particleToken"`
}

type AuthResponse struct {
    Success bool   `json:"success"`
    Data    struct {
        Token    string `json:"token"`
        Username string `json:"username"`
        Email    string `json:"email"`
    } `json:"data"`
    Error string `json:"error"`
}

func LoginHandler(c *fiber.Ctx) error {
    var req LoginRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{
            "success": false,
            "error":   "Invalid request",
        })
    }

    // Call auth API
    jsonData, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", apiEndpoint+"/auth/login", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{
            "success": false,
            "error":   "Failed to authenticate",
        })
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode != 200 {
        var errorResp AuthResponse
        json.Unmarshal(body, &errorResp)
        return c.Status(resp.StatusCode).JSON(fiber.Map{
            "success": false,
            "error":   errorResp.Error,
        })
    }

    var authResp AuthResponse
    json.Unmarshal(body, &authResp)

    // Set cookie
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    authResp.Data.Token,
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        Secure:   true,
        SameSite: "Strict",
    })

    c.Cookie(&fiber.Cookie{
        Name:     "username",
        Value:    authResp.Data.Username,
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: false,
    })

    return c.JSON(fiber.Map{
        "success": true,
        "redirect": "/dashboard",
    })
}

func RegisterHandler(c *fiber.Ctx) error {
    var req RegisterRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{
            "success": false,
            "error":   "Invalid request",
        })
    }

    // Call auth API
    jsonData, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", apiEndpoint+"/auth/register", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{
            "success": false,
            "error":   "Failed to register",
        })
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode != 201 {
        var errorResp AuthResponse
        json.Unmarshal(body, &errorResp)
        return c.Status(resp.StatusCode).JSON(fiber.Map{
            "success": false,
            "error":   errorResp.Error,
        })
    }

    var authResp AuthResponse
    json.Unmarshal(body, &authResp)

    // Set cookie
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    authResp.Data.Token,
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        Secure:   true,
        SameSite: "Strict",
    })

    c.Cookie(&fiber.Cookie{
        Name:     "username",
        Value:    authResp.Data.Username,
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: false,
    })

    return c.JSON(fiber.Map{
        "success": true,
        "redirect": "/dashboard",
    })
}

func LogoutHandler(c *fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    "",
        Expires:  time.Now().Add(-1 * time.Hour),
        HTTPOnly: true,
    })

    c.Cookie(&fiber.Cookie{
        Name:     "username",
        Value:    "",
        Expires:  time.Now().Add(-1 * time.Hour),
        HTTPOnly: false,
    })

    return c.Redirect("/")
}
