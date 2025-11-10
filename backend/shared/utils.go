package shared

import (
    "encoding/base64"
    "encoding/json"
    "os"

    "github.com/aws/aws-lambda-go/events"
)

// GetEnv retrieves an environment variable or returns a default value
func GetEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvOrDefault(key, defaultValue string) string {
    return GetEnv(key, defaultValue)
}

// CreateResponse creates a standard API Gateway response
func CreateResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
    jsonBody, _ := json.Marshal(body)
    return events.APIGatewayProxyResponse{
        StatusCode: statusCode,
        Headers: map[string]string{
            "Content-Type":                "application/json",
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type,Authorization",
        },
        Body: string(jsonBody),
    }
}

// CreateSuccessResponse creates a success response
func CreateSuccessResponse(statusCode int, data interface{}) events.APIGatewayProxyResponse {
    response := APIResponse{
        Success: true,
        Data:    data,
    }
    return CreateResponse(statusCode, response)
}

// CreateErrorResponse creates an error response
func CreateErrorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
    response := APIResponse{
        Success: false,
        Error:   message,
    }
    return CreateResponse(statusCode, response)
}

// GetAuthToken extracts the auth token from the request headers
func GetAuthToken(request events.APIGatewayProxyRequest) string {
    auth := request.Headers["Authorization"]
    if auth == "" {
        auth = request.Headers["authorization"]
    }
    if len(auth) > 7 && auth[:7] == "Bearer " {
        return auth[7:]
    }
    return ""
}

// ValidateAuth validates the auth token and returns the username
func ValidateAuth(request events.APIGatewayProxyRequest) (string, error) {
    token := GetAuthToken(request)
    if token == "" {
        return "", nil
    }

    claims, err := ValidateToken(token)
    if err != nil {
        return "", err
    }

    return claims.Username, nil
}

// GetRequestBody returns the request body, decoding from base64 if needed
func GetRequestBody(request events.APIGatewayProxyRequest) string {
    body := request.Body

    // If body is base64 encoded, decode it
    if request.IsBase64Encoded {
        decoded, err := base64.StdEncoding.DecodeString(body)
        if err == nil {
            return string(decoded)
        }
    }

    return body
}
