package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
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

    log.Printf("=== Auth Handler Called ===")
    log.Printf("Path: %s", path)
    log.Printf("Method: %s", method)
    log.Printf("Source IP: %s", request.RequestContext.Identity.SourceIP)
    log.Printf("User Agent: %s", request.Headers["User-Agent"])

    switch {
    case path == "/api/auth/login" && method == "POST":
        log.Println("Routing to handleLogin")
        return handleLogin(ctx, request)
    case path == "/api/auth/register" && method == "POST":
        log.Println("Routing to handleRegister")
        return handleRegister(ctx, request)
    case path == "/api/auth/validate" && method == "POST":
        log.Println("Routing to handleValidate")
        return handleValidate(ctx, request)
    case path == "/api/settings/particle" && method == "POST":
        log.Println("Routing to handleUpdateParticleSettings")
        return handleUpdateParticleSettings(ctx, request)
    default:
        log.Printf("No matching route for path: %s, method: %s", path, method)
        return shared.CreateErrorResponse(404, "Not found"), nil
    }
}

func handleLogin(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Println("=== handleLogin: Starting ===")

    var loginReq shared.LoginRequest
    body := shared.GetRequestBody(request)
    log.Printf("handleLogin: Request body length: %d bytes", len(body))

    if err := json.Unmarshal([]byte(body), &loginReq); err != nil {
        log.Printf("handleLogin: Failed to unmarshal request: %v", err)
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    log.Printf("handleLogin: Login attempt for username: %s", loginReq.Username)

    // Get user from database
    key, _ := attributevalue.MarshalMap(map[string]string{
        "username": loginReq.Username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, key, &user); err != nil {
        log.Printf("handleLogin: Database error fetching user: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if user.Username == "" {
        log.Printf("handleLogin: User not found: %s", loginReq.Username)
        return shared.CreateErrorResponse(401, "Invalid credentials"), nil
    }

    log.Printf("handleLogin: User found, validating password for: %s", user.Username)

    // Validate password
    if !shared.CheckPasswordHash(loginReq.Password, user.PasswordHash) {
        log.Printf("handleLogin: Password validation failed for user: %s", user.Username)
        return shared.CreateErrorResponse(401, "Invalid credentials"), nil
    }

    log.Printf("handleLogin: Password validated successfully for user: %s", user.Username)

    // Check if password needs re-hashing (migration from cost 14 to 10)
    if shared.NeedsRehash(user.PasswordHash) {
        log.Printf("handleLogin: Migrating password hash for user: %s", user.Username)
        newHash, err := shared.HashPassword(loginReq.Password)
        if err == nil {
            user.PasswordHash = newHash
            user.UpdatedAt = time.Now()
            if err := shared.PutItem(ctx, usersTable, user); err != nil {
                log.Printf("handleLogin: Failed to update user password hash: %v", err)
                // Continue login even if update fails
            } else {
                log.Printf("handleLogin: Successfully migrated password hash for user: %s", user.Username)
            }
        } else {
            log.Printf("handleLogin: Failed to generate new hash for migration: %v", err)
        }
    }

    // Create session
    userAgent := request.Headers["User-Agent"]
    ipAddress := request.RequestContext.Identity.SourceIP
    log.Printf("handleLogin: Creating session for user: %s from IP: %s", user.Username, ipAddress)

    session, err := shared.CreateSession(ctx, user.Username, userAgent, ipAddress)
    if err != nil {
        log.Printf("handleLogin: Failed to create session: %v", err)
        return shared.CreateErrorResponse(500, "Failed to create session"), nil
    }

    log.Printf("handleLogin: Login successful for user: %s", user.Username)

    response := shared.LoginResponse{
        Token:    session.SessionID,
        Username: user.Username,
    }

    return shared.CreateSuccessResponse(200, response), nil
}

func handleRegister(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Println("=== handleRegister: Starting ===")

    var registerReq struct {
        Username string `json:"username"`
        Password string `json:"password"`
        Email    string `json:"email,omitempty"`
    }

    body := shared.GetRequestBody(request)
    log.Printf("handleRegister: Request body length: %d bytes", len(body))

    if err := json.Unmarshal([]byte(body), &registerReq); err != nil {
        log.Printf("handleRegister: Failed to unmarshal request: %v", err)
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    log.Printf("handleRegister: Registration attempt for username: %s", registerReq.Username)

    // Validate input
    if registerReq.Username == "" || registerReq.Password == "" {
        log.Println("handleRegister: Missing username or password")
        return shared.CreateErrorResponse(400, "Username and password are required"), nil
    }

    // Check if user already exists
    log.Printf("handleRegister: Checking if username exists: %s", registerReq.Username)
    key, _ := attributevalue.MarshalMap(map[string]string{
        "username": registerReq.Username,
    })

    var existingUser shared.User
    if err := shared.GetItem(ctx, usersTable, key, &existingUser); err != nil {
        log.Printf("handleRegister: Database error checking existing user: %v", err)
        return shared.CreateErrorResponse(500, "Database error"), nil
    }

    if existingUser.Username != "" {
        log.Printf("handleRegister: Username already exists: %s", registerReq.Username)
        return shared.CreateErrorResponse(409, "Username already exists"), nil
    }

    log.Printf("handleRegister: Username available, creating user: %s", registerReq.Username)

    // Hash password
    passwordHash, err := shared.HashPassword(registerReq.Password)
    if err != nil {
        log.Printf("handleRegister: Failed to hash password: %v", err)
        return shared.CreateErrorResponse(500, "Failed to hash password"), nil
    }

    // Create user
    user := shared.User{
        Username:     registerReq.Username,
        PasswordHash: passwordHash,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    log.Printf("handleRegister: Saving user to database: %s", user.Username)
    if err := shared.PutItem(ctx, usersTable, user); err != nil {
        log.Printf("handleRegister: Failed to create user in database: %v", err)
        return shared.CreateErrorResponse(500, "Failed to create user"), nil
    }

    log.Printf("handleRegister: User created successfully: %s", user.Username)

    // Create session
    userAgent := request.Headers["User-Agent"]
    ipAddress := request.RequestContext.Identity.SourceIP
    log.Printf("handleRegister: Creating session for new user: %s from IP: %s", user.Username, ipAddress)

    session, err := shared.CreateSession(ctx, user.Username, userAgent, ipAddress)
    if err != nil {
        log.Printf("handleRegister: Failed to create session: %v", err)
        return shared.CreateErrorResponse(500, "Failed to create session"), nil
    }

    log.Printf("handleRegister: Registration successful for user: %s", user.Username)

    response := shared.LoginResponse{
        Token:    session.SessionID,
        Username: user.Username,
    }

    return shared.CreateSuccessResponse(201, response), nil
}

func handleValidate(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    log.Println("=== handleValidate: Starting ===")

    username, err := shared.ValidateAuth(ctx, request)
    if err != nil {
        log.Printf("handleValidate: Auth validation failed: %v", err)
        return shared.CreateErrorResponse(401, "Invalid session"), nil
    }

    if username == "" {
        log.Println("handleValidate: No session provided or session invalid")
        return shared.CreateErrorResponse(401, "No session provided"), nil
    }

    log.Printf("handleValidate: Session validated successfully for user: %s", username)

    return shared.CreateSuccessResponse(200, map[string]string{
        "username": username,
        "valid":    "true",
    }), nil
}

func handleUpdateParticleSettings(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Validate authentication
    username, err := shared.ValidateAuth(ctx, request)
    if err != nil {
        log.Printf("UpdateParticleSettings: Auth validation failed: %v", err)
        return shared.CreateErrorResponse(401, "Unauthorized"), nil
    }

    log.Printf("UpdateParticleSettings: User %s updating particle token", username)

    var updateReq struct {
        ParticleToken string `json:"particleToken"`
    }

    body := shared.GetRequestBody(request)
    log.Printf("UpdateParticleSettings: Request body: %s", body)

    if err := json.Unmarshal([]byte(body), &updateReq); err != nil {
        log.Printf("UpdateParticleSettings: Failed to parse request: %v", err)
        return shared.CreateErrorResponse(400, "Invalid request body"), nil
    }

    if updateReq.ParticleToken == "" {
        log.Println("UpdateParticleSettings: Token is empty")
        return shared.CreateErrorResponse(400, "Particle token is required"), nil
    }

    log.Printf("UpdateParticleSettings: Token length: %d", len(updateReq.ParticleToken))

    // Get user from database
    key, _ := attributevalue.MarshalMap(map[string]string{
        "username": username,
    })

    var user shared.User
    if err := shared.GetItem(ctx, usersTable, key, &user); err != nil {
        log.Printf("UpdateParticleSettings: Failed to get user: %v", err)
        return shared.CreateErrorResponse(500, "Database error getting user"), nil
    }

    if user.Username == "" {
        log.Printf("UpdateParticleSettings: User %s not found", username)
        return shared.CreateErrorResponse(404, "User not found"), nil
    }

    log.Printf("UpdateParticleSettings: Found user %s, updating token", username)

    // Update particle token
    user.ParticleToken = updateReq.ParticleToken
    user.UpdatedAt = time.Now()

    log.Printf("UpdateParticleSettings: Attempting to save user to DynamoDB")
    if err := shared.PutItem(ctx, usersTable, user); err != nil {
        log.Printf("UpdateParticleSettings: Failed to update user in DynamoDB: %v", err)
        return shared.CreateErrorResponse(500, fmt.Sprintf("Failed to update settings: %v", err)), nil
    }

    log.Printf("UpdateParticleSettings: Successfully updated token for user %s", username)
    return shared.CreateSuccessResponse(200, map[string]string{
        "message": "Particle token updated successfully",
    }), nil
}

func main() {
    lambda.Start(handler)
}
