package main

import (
    "context"
    "encoding/json"
    "os"
    "time"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"

    "candle-lights/backend/shared"
)

var usersTable = os.Getenv("USERS_TABLE")

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    path := request.Path
    method := request.HTTPMethod

    switch {
    case path == "/auth/login" && method == "POST":
        return handleLogin(ctx, request)
    case path == "/auth/register" && method == "POST":
        return handleRegister(ctx, request)
    case path == "/auth/validate" && method == "POST":
        return handleValidate(ctx, request)
    default:
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

func handleLogin(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var loginReq shared.LoginRequest
    if err := json.Unmarshal([]byte(request.Body), &loginReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Get user from database
    key, _ := attributevalue.MarshalMap(map[string]string{
        "username": loginReq.Username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, key, &user); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.Username == "" {
        return shared.CreateErrorResponse(401, "Invalid credentials"), nil
    }

    // Validate password
    if !shared.CheckPasswordHash(loginReq.Password, user.PasswordHash) {
        return shared.CreateErrorResponse(401, "Invalid credentials"), nil
    }

    // Generate JWT token
    token, err := shared.GenerateToken(user.Username)
    if err != nil {
        return shared.CreateErrorResponse(500, "Failed to generate token"), nil
    }

    response := shared.LoginResponse{
        Token:    token,
        Username: user.Username,
    }

    return shared.CreateSuccessResponse(200, response), nil
}

func handleRegister(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var registerReq struct {
        Username         string `json:"username"`
        Password         string `json:"password"`
        ParticleUsername string `json:"particleUsername,omitempty"`
        ParticleToken    string `json:"particleToken,omitempty"`
    }

    if err := json.Unmarshal([]byte(request.Body), &registerReq); err != nil {
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    // Validate input
    if registerReq.Username == "" || registerReq.Password == "" {
        return shared.CreateErrorResponse(400, "Username and password are required"), nil
    }

    // Check if user already exists
    key, _ := attributevalue.MarshalMap(map[string]string{
        "username": registerReq.Username,
    })

    var existingUser shared.User
    if err := shared.GetItem(ctx, usersTable, key, &existingUser); err != nil {
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if existingUser.Username != "" {
        return shared.CreateErrorResponse(409, "Username already exists"), nil
    }

    // Hash password
    passwordHash, err := shared.HashPassword(registerReq.Password)
    if err != nil {
        return shared.CreateErrorResponse(500, "Failed to hash password"), nil
    }

    // Create user
    user := shared.User{
        Username:         registerReq.Username,
        PasswordHash:     passwordHash,
        ParticleUsername: registerReq.ParticleUsername,
        ParticleToken:    registerReq.ParticleToken,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }

    if err := shared.PutItem(ctx, usersTable, user); err != nil {
        return shared.CreateErrorResponse(500, "Failed to create user"), nil
    }

    // Generate token
    token, err := shared.GenerateToken(user.Username)
    if err != nil {
        return shared.CreateErrorResponse(500, "Failed to generate token"), nil
    }

    response := shared.LoginResponse{
        Token:    token,
        Username: user.Username,
    }

    return shared.CreateSuccessResponse(201, response), nil
}

func handleValidate(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    username, err := shared.ValidateAuth(request)
    if err != nil {
        return shared.CreateErrorResponse(401, "Invalid token"), nil
    }

    if username == "" {
        return shared.CreateErrorResponse(401, "No token provided"), nil
    }

    return shared.CreateSuccessResponse(200, map[string]string{
        "username": username,
        "valid":    "true",
    }), nil
}

func main() {
    lambda.Start(handler)
}
