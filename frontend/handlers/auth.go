package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log.Println("LoginHandler: Received login request")

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("LoginHandler: Failed to parse request body: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request",
		})
	}

	log.Printf("LoginHandler: Attempting to login user: %s", req.Username)

	// Call backend auth API with correct path: /api/auth/login
	apiURL := apiEndpoint + "/api/auth/login"
	log.Printf("LoginHandler: Calling backend API at: %s", apiURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		log.Printf("LoginHandler: Failed to marshal request: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Internal server error",
		})
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("LoginHandler: Failed to create HTTP request: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Internal server error",
		})
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("LoginHandler: Failed to call backend API: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to authenticate: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("LoginHandler: Failed to read response body: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read response",
		})
	}

	log.Printf("LoginHandler: Backend API response status: %d", resp.StatusCode)
	log.Printf("LoginHandler: Backend API response body: %s", string(body))

	if resp.StatusCode != 200 {
		var errorResp AuthResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			log.Printf("LoginHandler: Failed to parse error response: %v", err)
			return c.Status(resp.StatusCode).JSON(fiber.Map{
				"success": false,
				"error":   "Authentication failed",
			})
		}

		log.Printf("LoginHandler: Authentication failed: %s", errorResp.Error)
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   errorResp.Error,
		})
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		log.Printf("LoginHandler: Failed to parse success response: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to parse response",
		})
	}

	if !authResp.Success || authResp.Data.Token == "" {
		log.Printf("LoginHandler: Invalid response from backend: success=%v, token=%s", authResp.Success, authResp.Data.Token)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid response from server",
		})
	}

	log.Printf("LoginHandler: Login successful for user: %s", authResp.Data.Username)

	// Set secure HTTP-only cookie for token
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    authResp.Data.Token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // Allow both HTTP and HTTPS for better compatibility
		SameSite: "Lax",  // Allow OAuth redirects while maintaining CSRF protection
		Path:     "/",
	})

	// Set username cookie (readable by JavaScript)
	c.Cookie(&fiber.Cookie{
		Name:     "username",
		Value:    authResp.Data.Username,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: false,
		Secure:   false, // Allow both HTTP and HTTPS for better compatibility
		SameSite: "Lax",  // Allow OAuth redirects while maintaining CSRF protection
		Path:     "/",
	})

	log.Printf("LoginHandler: Cookies set, returning success response")

	return c.JSON(fiber.Map{
		"success":  true,
		"redirect": "/dashboard",
	})
}

func RegisterHandler(c *fiber.Ctx) error {
	log.Println("RegisterHandler: Received registration request")

	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("RegisterHandler: Failed to parse request body: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request",
		})
	}

	log.Printf("RegisterHandler: Attempting to register user: %s", req.Username)

	// Call backend auth API with correct path: /api/auth/register
	apiURL := apiEndpoint + "/api/auth/register"
	log.Printf("RegisterHandler: Calling backend API at: %s", apiURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		log.Printf("RegisterHandler: Failed to marshal request: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Internal server error",
		})
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("RegisterHandler: Failed to create HTTP request: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Internal server error",
		})
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("RegisterHandler: Failed to call backend API: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to register: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("RegisterHandler: Failed to read response body: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read response",
		})
	}

	log.Printf("RegisterHandler: Backend API response status: %d", resp.StatusCode)
	log.Printf("RegisterHandler: Backend API response body: %s", string(body))

	if resp.StatusCode != 201 {
		var errorResp AuthResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			log.Printf("RegisterHandler: Failed to parse error response: %v", err)
			return c.Status(resp.StatusCode).JSON(fiber.Map{
				"success": false,
				"error":   "Registration failed",
			})
		}

		log.Printf("RegisterHandler: Registration failed: %s", errorResp.Error)
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   errorResp.Error,
		})
	}

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		log.Printf("RegisterHandler: Failed to parse success response: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to parse response",
		})
	}

	log.Printf("RegisterHandler: Registration successful for user: %s", authResp.Data.Username)

	// Set secure HTTP-only cookie for token
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    authResp.Data.Token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // Allow both HTTP and HTTPS for better compatibility
		SameSite: "Lax",  // Allow OAuth redirects while maintaining CSRF protection
		Path:     "/",
	})

	// Set username cookie (readable by JavaScript)
	c.Cookie(&fiber.Cookie{
		Name:     "username",
		Value:    authResp.Data.Username,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: false,
		Secure:   false, // Allow both HTTP and HTTPS for better compatibility
		SameSite: "Lax",  // Allow OAuth redirects while maintaining CSRF protection
		Path:     "/",
	})

	log.Printf("RegisterHandler: Cookies set, returning success response")

	return c.JSON(fiber.Map{
		"success":  true,
		"redirect": "/dashboard",
	})
}

func LogoutHandler(c *fiber.Ctx) error {
	log.Println("LogoutHandler: Logging out user")

	// Clear token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
		Path:     "/",
	})

	// Clear username cookie
	c.Cookie(&fiber.Cookie{
		Name:     "username",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: false,
		Path:     "/",
	})

	log.Println("LogoutHandler: Cookies cleared, redirecting to home")

	return c.Redirect("/")
}
