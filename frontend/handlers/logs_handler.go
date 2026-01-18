package handlers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/gofiber/fiber/v2"
)

// LogsHandler renders the logs page
func LogsHandler(c *fiber.Ctx) error {
	username := c.Locals("username").(string)
	return c.Render("templates/logs", fiber.Map{
		"Title":    "Logs",
		"Username": username,
	})
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
	LogStream string `json:"logStream"`
}

// GetLogsHandler fetches CloudWatch logs for a Lambda function
func GetLogsHandler(c *fiber.Ctx) error {
	functionName := c.Query("function")
	hoursStr := c.Query("hours", "1")

	if functionName == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "function parameter is required",
		})
	}

	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours < 1 || hours > 168 {
		hours = 1
	}

	// Build log group name from stack name and function
	stackName := os.Getenv("STACK_NAME")
	if stackName == "" {
		stackName = "candle-lights-prod"
	}
	logGroupName := fmt.Sprintf("/aws/lambda/%s-%s", stackName, functionName)

	// Load AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to load AWS config: " + err.Error(),
		})
	}

	client := cloudwatchlogs.NewFromConfig(cfg)

	// Calculate time range
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)

	// Fetch log events
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(logGroupName),
		StartTime:    aws.Int64(startTime.UnixMilli()),
		EndTime:      aws.Int64(endTime.UnixMilli()),
		Limit:        aws.Int32(500),
	}

	var allEvents []LogEntry

	paginator := cloudwatchlogs.NewFilterLogEventsPaginator(client, input)

	// Limit to 3 pages to avoid overwhelming response
	pageCount := 0
	for paginator.HasMorePages() && pageCount < 3 {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			// Log group might not exist yet
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				return c.JSON(fiber.Map{
					"success": true,
					"data": fiber.Map{
						"logs":         []LogEntry{},
						"logGroupName": logGroupName,
						"hours":        hours,
						"message":      "No logs found - log group does not exist yet",
					},
				})
			}
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to fetch logs: " + err.Error(),
			})
		}

		for _, event := range page.Events {
			allEvents = append(allEvents, LogEntry{
				Timestamp: aws.ToInt64(event.Timestamp),
				Message:   aws.ToString(event.Message),
				LogStream: aws.ToString(event.LogStreamName),
			})
		}
		pageCount++
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp > allEvents[j].Timestamp
	})

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"logs":         allEvents,
			"logGroupName": logGroupName,
			"hours":        hours,
			"count":        len(allEvents),
		},
	})
}

// ListLogFunctionsHandler returns the list of available Lambda functions
func ListLogFunctionsHandler(c *fiber.Ctx) error {
	functions := []map[string]string{
		{"id": "AuthFunction", "name": "Auth", "description": "Login/Register/Sessions"},
		{"id": "PatternsFunction", "name": "Patterns", "description": "Pattern CRUD & Effects"},
		{"id": "DevicesFunction", "name": "Devices", "description": "Device Management"},
		{"id": "ParticleFunction", "name": "Particle", "description": "Particle.io Commands"},
		{"id": "GlowBlasterFunction", "name": "GlowBlaster", "description": "AI Pattern Generation"},
		{"id": "OAuthFunction", "name": "OAuth", "description": "Particle OAuth Flow"},
		{"id": "FrontendFunction", "name": "Frontend", "description": "Web UI & Routing"},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    functions,
	})
}
