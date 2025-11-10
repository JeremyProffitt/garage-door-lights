package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupTestApp() *fiber.App {
	app := fiber.New()
	setupRoutes(app)
	return app
}

func TestLoginRouteExists(t *testing.T) {
	app := setupTestApp()

	// Test that /api/auth/login route exists and accepts POST
	loginReq := map[string]string{
		"username": "testuser",
		"password": "testpass",
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	// We expect either 200, 401, or 500, but NOT 404
	assert.NotEqual(t, 404, resp.StatusCode, "Login route should exist")
}

func TestRegisterRouteExists(t *testing.T) {
	app := setupTestApp()

	// Test that /api/auth/register route exists and accepts POST
	registerReq := map[string]string{
		"username": "testuser",
		"password": "testpass",
		"email":    "test@example.com",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	// We expect either 200, 201, 401, or 500, but NOT 404
	assert.NotEqual(t, 404, resp.StatusCode, "Register route should exist")
}

func TestAPIRoutesExist(t *testing.T) {
	app := setupTestApp()

	tests := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/api/patterns", "Get Patterns"},
		{"POST", "/api/patterns", "Create Pattern"},
		{"GET", "/api/devices", "Get Devices"},
		{"POST", "/api/devices", "Create Device"},
		{"POST", "/api/particle/command", "Send Command"},
		{"POST", "/api/particle/devices/refresh", "Refresh Devices"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)

			resp, err := app.Test(req, -1)

			assert.NoError(t, err)
			// We expect 401 (unauthorized) or 500, but NOT 404
			assert.NotEqual(t, 404, resp.StatusCode, "%s route should exist", tt.name)
		})
	}
}

func TestLoginPageRendersCorrectly(t *testing.T) {
	t.Skip("Skipping template rendering test - requires embedded files")
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/login", nil)
	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "Login page should render")
}

func TestDashboardRequiresAuth(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/dashboard", nil)
	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	// Should redirect to login (302) since no auth cookie
	assert.Equal(t, 302, resp.StatusCode, "Dashboard should require authentication")
}
