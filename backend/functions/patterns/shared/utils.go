package shared

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "log"
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

// GetSessionID extracts the session ID from cookies or Authorization header
func GetSessionID(request events.APIGatewayProxyRequest) string {
    // First try to get from Authorization header (Bearer token)
    auth := request.Headers["Authorization"]
    if auth == "" {
        auth = request.Headers["authorization"]
    }
    if len(auth) > 7 && auth[:7] == "Bearer " {
        return auth[7:]
    }

    // Try to get from Cookie header
    cookie := request.Headers["Cookie"]
    if cookie == "" {
        cookie = request.Headers["cookie"]
    }

    if cookie != "" {
        // Parse cookies (simple parsing for session_id cookie)
        cookiePairs := parseCookies(cookie)
        if sessionID, ok := cookiePairs["session_id"]; ok {
            return sessionID
        }
    }

    return ""
}

// parseCookies parses a cookie header string into a map
func parseCookies(cookieHeader string) map[string]string {
    cookies := make(map[string]string)
    pairs := splitCookies(cookieHeader)

    for _, pair := range pairs {
        parts := splitPair(pair)
        if len(parts) == 2 {
            cookies[trimSpace(parts[0])] = trimSpace(parts[1])
        }
    }

    return cookies
}

// splitCookies splits cookie header by semicolon
func splitCookies(s string) []string {
    var result []string
    current := ""

    for _, char := range s {
        if char == ';' {
            if current != "" {
                result = append(result, current)
            }
            current = ""
        } else {
            current += string(char)
        }
    }

    if current != "" {
        result = append(result, current)
    }

    return result
}

// splitPair splits a key=value pair
func splitPair(s string) []string {
    for i, char := range s {
        if char == '=' {
            return []string{s[:i], s[i+1:]}
        }
    }
    return []string{s}
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
    start := 0
    end := len(s)

    for start < end && (s[start] == ' ' || s[start] == '\t') {
        start++
    }

    for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
        end--
    }

    return s[start:end]
}

// ValidateAuth validates the session and returns the username
func ValidateAuth(ctx context.Context, request events.APIGatewayProxyRequest) (string, error) {
    sessionID := GetSessionID(request)
    if sessionID == "" {
        log.Println("ValidateAuth: No session ID found in request")
        log.Printf("ValidateAuth: Headers: %+v", request.Headers)
        log.Printf("ValidateAuth: Cookies: %+v", request.Headers["Cookie"])
        return "", nil
    }

    log.Printf("ValidateAuth: Session ID found (first 20 chars): %s...", safeDisplay(sessionID, 20))

    session, err := GetSession(ctx, sessionID)
    if err != nil {
        log.Printf("ValidateAuth: Session lookup failed: %v", err)
        return "", err
    }

    if session == nil {
        log.Println("ValidateAuth: Session not found or expired")
        return "", nil
    }

    log.Printf("ValidateAuth: Session validated successfully for user: %s", session.Username)
    return session.Username, nil
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
