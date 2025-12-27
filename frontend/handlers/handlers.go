package handlers

import (
    "bytes"
    "io"
    "net/http"
    "os"

    "github.com/gofiber/fiber/v2"
)

var apiEndpoint = os.Getenv("API_ENDPOINT")

// indexHandler renders the homepage
func IndexHandler(c *fiber.Ctx) error {
    return c.Render("templates/index", fiber.Map{
        "Title": "Candle Lights Controller",
    })
}

// loginPageHandler renders the login page
func LoginPageHandler(c *fiber.Ctx) error {
    return c.Render("templates/login", fiber.Map{
        "Title": "Login",
    })
}

// registerPageHandler renders the registration page
func RegisterPageHandler(c *fiber.Ctx) error {
    return c.Render("templates/register", fiber.Map{
        "Title": "Register",
    })
}

// dashboardHandler renders the dashboard
func DashboardHandler(c *fiber.Ctx) error {
    username := c.Locals("username").(string)
    return c.Render("templates/dashboard", fiber.Map{
        "Title":    "Dashboard",
        "Username": username,
    })
}

// patternsHandler renders the patterns page
func PatternsHandler(c *fiber.Ctx) error {
    username := c.Locals("username").(string)
    return c.Render("templates/patterns", fiber.Map{
        "Title":    "Light Patterns",
        "Username": username,
    })
}

// devicesHandler renders the devices page
func DevicesHandler(c *fiber.Ctx) error {
    username := c.Locals("username").(string)
    return c.Render("templates/devices", fiber.Map{
        "Title":    "Devices",
        "Username": username,
    })
}

// settingsHandler renders the settings page
func SettingsHandler(c *fiber.Ctx) error {
    username := c.Locals("username").(string)
    return c.Render("templates/settings", fiber.Map{
        "Title":    "Settings",
        "Username": username,
    })
}

// glowBlasterHandler renders the Glow Blaster AI pattern creation page
func GlowBlasterHandler(c *fiber.Ctx) error {
    username := c.Locals("username").(string)
    return c.Render("templates/glowblaster", fiber.Map{
        "Title":    "Glow Blaster",
        "Username": username,
    })
}

// API handlers that proxy to backend Lambda functions

func GetPatternsHandler(c *fiber.Ctx) error {
    return proxyRequest(c, "GET", "/api/patterns", nil)
}

func CreatePatternHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/patterns", body)
}

func UpdatePatternHandler(c *fiber.Ctx) error {
    id := c.Params("id")
    body := c.Body()
    return proxyRequest(c, "PUT", "/api/patterns/"+id, body)
}

func DeletePatternHandler(c *fiber.Ctx) error {
    id := c.Params("id")
    return proxyRequest(c, "DELETE", "/api/patterns/"+id, nil)
}

func GetDevicesHandler(c *fiber.Ctx) error {
    return proxyRequest(c, "GET", "/api/devices", nil)
}

func CreateDeviceHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/devices", body)
}

func AssignPatternHandler(c *fiber.Ctx) error {
    id := c.Params("id")
    body := c.Body()
    return proxyRequest(c, "PUT", "/api/devices/"+id+"/pattern", body)
}

func SendCommandHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/particle/command", body)
}

func RefreshDevicesHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/particle/devices/refresh", body)
}

func UpdateParticleSettingsHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/settings/particle", body)
}

func ValidateParticleTokenHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/particle/validate-token", body)
}

func ParticleOAuthInitiateHandler(c *fiber.Ctx) error {
    body := c.Body()
    return proxyRequest(c, "POST", "/api/particle/oauth/initiate", body)
}

func proxyRequest(c *fiber.Ctx, method, path string, body []byte) error {
    sessionID := c.Cookies("session_id")
    if sessionID == "" {
        return c.Status(401).JSON(fiber.Map{
            "success": false,
            "error":   "Unauthorized - No session",
        })
    }

    url := apiEndpoint + path

    var req *http.Request
    var err error

    if body != nil {
        req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
    } else {
        req, err = http.NewRequest(method, url, nil)
    }

    if err != nil {
        return c.Status(500).JSON(fiber.Map{
            "success": false,
            "error":   "Failed to create request",
        })
    }

    // Pass session ID as Bearer token in Authorization header
    req.Header.Set("Authorization", "Bearer "+sessionID)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{
            "success": false,
            "error":   "Failed to send request",
        })
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{
            "success": false,
            "error":   "Failed to read response",
        })
    }

    c.Set("Content-Type", "application/json")
    return c.Status(resp.StatusCode).Send(respBody)
}
