package handlers

import (
	"context"
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

// LogFunction represents a Lambda function for the dropdown
type LogFunction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	LogGroup    string `json:"logGroup"`
}

// functionMeta maps function name patterns to display info
var functionMeta = map[string]struct {
	name        string
	description string
}{
	"AuthFunction":       {"Auth", "Login/Register/Sessions"},
	"PatternsFunction":   {"Patterns", "Pattern CRUD & Effects"},
	"DevicesFunction":    {"Devices", "Device Management"},
	"ParticleFunction":   {"Particle", "Particle.io Commands"},
	"GlowBlasterFunction": {"GlowBlaster", "AI Pattern Generation"},
	"OAuthFunction":      {"OAuth", "Particle OAuth Flow"},
	"FrontendFunction":   {"Frontend", "Web UI & Routing"},
	"AlexaFunction":      {"Alexa", "Alexa Smart Home Skill"},
}

// GetLogsHandler fetches CloudWatch logs for a Lambda function
func GetLogsHandler(c *fiber.Ctx) error {
	logGroupName := c.Query("logGroup")
	hoursStr := c.Query("hours", "1")

	if logGroupName == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "logGroup parameter is required",
		})
	}

	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours < 1 || hours > 168 {
		hours = 1
	}

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

// ListLogFunctionsHandler discovers and returns available Lambda log groups
func ListLogFunctionsHandler(c *fiber.Ctx) error {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to load AWS config: " + err.Error(),
		})
	}

	client := cloudwatchlogs.NewFromConfig(cfg)

	// List log groups with our prefix
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String("/aws/lambda/candle-lights-"),
		Limit:              aws.Int32(50),
	}

	resp, err := client.DescribeLogGroups(ctx, input)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list log groups: " + err.Error(),
		})
	}

	var functions []LogFunction

	for _, lg := range resp.LogGroups {
		logGroupName := aws.ToString(lg.LogGroupName)

		// Extract function name from log group (e.g., /aws/lambda/candle-lights-AuthFunction-xyz -> AuthFunction)
		for funcPattern, meta := range functionMeta {
			if strings.Contains(logGroupName, funcPattern) {
				functions = append(functions, LogFunction{
					ID:          funcPattern,
					Name:        meta.name,
					Description: meta.description,
					LogGroup:    logGroupName,
				})
				break
			}
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Name < functions[j].Name
	})

	return c.JSON(fiber.Map{
		"success": true,
		"data":    functions,
	})
}
