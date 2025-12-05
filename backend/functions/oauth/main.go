package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"

	"candle-lights/backend/shared"
)

var (
	usersTable       = os.Getenv("USERS_TABLE")
	alexaClientID    = os.Getenv("ALEXA_CLIENT_ID")
	alexaClientSecret = os.Getenv("ALEXA_CLIENT_SECRET")
	domainName       = os.Getenv("DOMAIN_NAME")
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== OAuth Handler Called ===")
	log.Printf("Path: %s", request.Path)
	log.Printf("Method: %s", request.HTTPMethod)

	path := request.Path
	method := request.HTTPMethod

	switch {
	case path == "/oauth/authorize" && method == "GET":
		return handleAuthorizeGet(ctx, request)
	case path == "/oauth/authorize" && method == "POST":
		return handleAuthorizePost(ctx, request)
	case path == "/oauth/token" && method == "POST":
		return handleToken(ctx, request)
	default:
		return shared.CreateErrorResponse(404, "Not found"), nil
	}
}

// handleAuthorizeGet shows the login page for OAuth authorization
func handleAuthorizeGet(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleAuthorizeGet ===")

	// Extract OAuth parameters
	clientID := request.QueryStringParameters["client_id"]
	redirectURI := request.QueryStringParameters["redirect_uri"]
	state := request.QueryStringParameters["state"]
	responseType := request.QueryStringParameters["response_type"]
	scope := request.QueryStringParameters["scope"]

	log.Printf("OAuth params: client_id=%s, redirect_uri=%s, state=%s, response_type=%s",
		clientID, redirectURI, state, responseType)

	// Validate required parameters
	if clientID == "" || redirectURI == "" || responseType != "code" {
		return createHTMLResponse(400, renderErrorPage("Invalid OAuth request parameters")), nil
	}

	// Validate client ID
	if clientID != alexaClientID {
		log.Printf("Invalid client_id: %s (expected: %s)", clientID, alexaClientID)
		return createHTMLResponse(400, renderErrorPage("Invalid client ID")), nil
	}

	// Render login page
	html := renderLoginPage(clientID, redirectURI, state, scope)
	return createHTMLResponse(200, html), nil
}

// handleAuthorizePost processes the login form submission
func handleAuthorizePost(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleAuthorizePost ===")

	// Parse form data
	body := shared.GetRequestBody(request)
	formData, err := url.ParseQuery(body)
	if err != nil {
		log.Printf("Failed to parse form data: %v", err)
		return createHTMLResponse(400, renderErrorPage("Invalid form data")), nil
	}

	username := formData.Get("username")
	password := formData.Get("password")
	clientID := formData.Get("client_id")
	redirectURI := formData.Get("redirect_uri")
	state := formData.Get("state")
	scope := formData.Get("scope")

	log.Printf("Login attempt: username=%s, client_id=%s", username, clientID)

	// Validate credentials
	if username == "" || password == "" {
		return createHTMLResponse(400, renderLoginPageWithError(
			clientID, redirectURI, state, scope, "Username and password are required")), nil
	}

	// Get user from database
	userKey, _ := attributevalue.MarshalMap(map[string]string{
		"username": username,
	})

	var user shared.User
	if err := shared.GetItem(ctx, usersTable, userKey, &user); err != nil {
		log.Printf("Database error: %v", err)
		return createHTMLResponse(500, renderErrorPage("Internal server error")), nil
	}

	if user.Username == "" {
		log.Printf("User not found: %s", username)
		return createHTMLResponse(401, renderLoginPageWithError(
			clientID, redirectURI, state, scope, "Invalid username or password")), nil
	}

	// Verify password
	if !shared.CheckPasswordHash(password, user.PasswordHash) {
		log.Printf("Invalid password for user: %s", username)
		return createHTMLResponse(401, renderLoginPageWithError(
			clientID, redirectURI, state, scope, "Invalid username or password")), nil
	}

	log.Printf("User authenticated successfully: %s", username)

	// Generate authorization code
	authCode, err := shared.GenerateAuthCode(ctx, username, clientID, redirectURI, scope)
	if err != nil {
		log.Printf("Failed to generate auth code: %v", err)
		return createHTMLResponse(500, renderErrorPage("Failed to generate authorization code")), nil
	}

	// Build redirect URL
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		log.Printf("Invalid redirect URI: %v", err)
		return createHTMLResponse(400, renderErrorPage("Invalid redirect URI")), nil
	}

	query := redirectURL.Query()
	query.Set("code", authCode.Code)
	if state != "" {
		query.Set("state", state)
	}
	redirectURL.RawQuery = query.Encode()

	log.Printf("Redirecting to: %s", redirectURL.String())

	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers: map[string]string{
			"Location":                     redirectURL.String(),
			"Access-Control-Allow-Origin":  "*",
		},
	}, nil
}

// handleToken handles the token exchange endpoint
func handleToken(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleToken ===")

	// Parse request body (can be JSON or form-encoded)
	body := shared.GetRequestBody(request)
	contentType := request.Headers["Content-Type"]
	if contentType == "" {
		contentType = request.Headers["content-type"]
	}

	var grantType, code, redirectURI, clientID, clientSecret, refreshToken string

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		formData, err := url.ParseQuery(body)
		if err != nil {
			log.Printf("Failed to parse form data: %v", err)
			return createTokenError("invalid_request", "Invalid request body"), nil
		}
		grantType = formData.Get("grant_type")
		code = formData.Get("code")
		redirectURI = formData.Get("redirect_uri")
		clientID = formData.Get("client_id")
		clientSecret = formData.Get("client_secret")
		refreshToken = formData.Get("refresh_token")
	} else {
		var tokenReq shared.TokenRequest
		if err := json.Unmarshal([]byte(body), &tokenReq); err != nil {
			log.Printf("Failed to parse JSON body: %v", err)
			return createTokenError("invalid_request", "Invalid request body"), nil
		}
		grantType = tokenReq.GrantType
		code = tokenReq.Code
		redirectURI = tokenReq.RedirectURI
		clientID = tokenReq.ClientID
		clientSecret = tokenReq.ClientSecret
		refreshToken = tokenReq.RefreshToken
	}

	log.Printf("Token request: grant_type=%s, client_id=%s", grantType, clientID)

	// Validate client credentials
	if clientID != alexaClientID || clientSecret != alexaClientSecret {
		log.Printf("Invalid client credentials")
		return createTokenError("invalid_client", "Invalid client credentials"), nil
	}

	switch grantType {
	case "authorization_code":
		return handleAuthorizationCodeGrant(ctx, code, redirectURI)
	case "refresh_token":
		return handleRefreshTokenGrant(ctx, refreshToken)
	default:
		return createTokenError("unsupported_grant_type", "Unsupported grant type"), nil
	}
}

func handleAuthorizationCodeGrant(ctx context.Context, code, redirectURI string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleAuthorizationCodeGrant ===")

	if code == "" {
		return createTokenError("invalid_request", "Authorization code is required"), nil
	}

	// Get and validate auth code
	authCode, err := shared.GetAuthCode(ctx, code)
	if err != nil {
		log.Printf("Failed to get auth code: %v", err)
		return createTokenError("server_error", "Internal server error"), nil
	}

	if authCode == nil {
		log.Printf("Auth code not found or expired")
		return createTokenError("invalid_grant", "Invalid or expired authorization code"), nil
	}

	// Validate redirect URI matches
	if authCode.RedirectURI != redirectURI {
		log.Printf("Redirect URI mismatch: expected=%s, got=%s", authCode.RedirectURI, redirectURI)
		return createTokenError("invalid_grant", "Redirect URI mismatch"), nil
	}

	// Delete the auth code (single use)
	shared.DeleteAuthCode(ctx, code)

	// Create access token
	token, accessToken, err := shared.CreateAccessToken(ctx, authCode.UserID, authCode.Scope)
	if err != nil {
		log.Printf("Failed to create access token: %v", err)
		return createTokenError("server_error", "Failed to create access token"), nil
	}

	// Build response
	expiresIn := int(time.Until(time.Unix(token.ExpiresAt, 0)).Seconds())
	response := shared.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: token.RefreshToken,
	}

	log.Printf("Token created successfully for user: %s", authCode.UserID)
	return createJSONResponse(200, response), nil
}

func handleRefreshTokenGrant(ctx context.Context, refreshToken string) (events.APIGatewayProxyResponse, error) {
	log.Printf("=== handleRefreshTokenGrant ===")

	if refreshToken == "" {
		return createTokenError("invalid_request", "Refresh token is required"), nil
	}

	// Refresh the token
	token, accessToken, err := shared.RefreshAccessToken(ctx, refreshToken)
	if err != nil {
		log.Printf("Failed to refresh token: %v", err)
		return createTokenError("server_error", "Internal server error"), nil
	}

	if token == nil {
		log.Printf("Refresh token not found")
		return createTokenError("invalid_grant", "Invalid refresh token"), nil
	}

	// Build response
	expiresIn := int(time.Until(time.Unix(token.ExpiresAt, 0)).Seconds())
	response := shared.TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: token.RefreshToken,
	}

	log.Printf("Token refreshed successfully")
	return createJSONResponse(200, response), nil
}

// Helper functions for responses

func createHTMLResponse(statusCode int, html string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                "text/html; charset=utf-8",
			"Access-Control-Allow-Origin": "*",
		},
		Body: html,
	}
}

func createJSONResponse(statusCode int, data interface{}) events.APIGatewayProxyResponse {
	jsonBody, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(jsonBody),
	}
}

func createTokenError(errorCode, description string) events.APIGatewayProxyResponse {
	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}
	jsonBody, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(jsonBody),
	}
}

// HTML Templates

const loginPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Link Your Account - Garage Lights</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            padding: 40px;
            width: 100%;
            max-width: 400px;
        }
        .logo {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo h1 {
            color: #333;
            font-size: 24px;
            margin-bottom: 8px;
        }
        .logo p {
            color: #666;
            font-size: 14px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            color: #333;
            font-weight: 500;
            margin-bottom: 8px;
            font-size: 14px;
        }
        input[type="text"],
        input[type="password"] {
            width: 100%;
            padding: 14px 16px;
            border: 2px solid #e1e1e1;
            border-radius: 8px;
            font-size: 16px;
            transition: border-color 0.2s;
        }
        input:focus {
            outline: none;
            border-color: #667eea;
        }
        .error {
            background: #fee;
            border: 1px solid #fcc;
            color: #c33;
            padding: 12px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        button {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        button:active {
            transform: translateY(0);
        }
        .alexa-notice {
            text-align: center;
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            color: #666;
            font-size: 12px;
        }
        .alexa-notice img {
            height: 20px;
            vertical-align: middle;
            margin-right: 6px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <h1>Garage Lights</h1>
            <p>Link your account with Alexa</p>
        </div>
        {{if .Error}}
        <div class="error">{{.Error}}</div>
        {{end}}
        <form method="POST" action="/oauth/authorize">
            <input type="hidden" name="client_id" value="{{.ClientID}}">
            <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
            <input type="hidden" name="state" value="{{.State}}">
            <input type="hidden" name="scope" value="{{.Scope}}">

            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required autocomplete="username">
            </div>

            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required autocomplete="current-password">
            </div>

            <button type="submit">Link Account</button>
        </form>
        <div class="alexa-notice">
            By linking, you allow Alexa to control your garage lights
        </div>
    </div>
</body>
</html>`

const errorPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error - Garage Lights</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            padding: 40px;
            text-align: center;
            max-width: 400px;
        }
        h1 {
            color: #c33;
            margin-bottom: 16px;
        }
        p {
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Error</h1>
        <p>{{.Message}}</p>
    </div>
</body>
</html>`

type loginPageData struct {
	ClientID    string
	RedirectURI string
	State       string
	Scope       string
	Error       string
}

type errorPageData struct {
	Message string
}

func renderLoginPage(clientID, redirectURI, state, scope string) string {
	return renderLoginPageWithError(clientID, redirectURI, state, scope, "")
}

func renderLoginPageWithError(clientID, redirectURI, state, scope, errorMsg string) string {
	tmpl, err := template.New("login").Parse(loginPageTemplate)
	if err != nil {
		return fmt.Sprintf("Template error: %v", err)
	}

	data := loginPageData{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		State:       state,
		Scope:       scope,
		Error:       errorMsg,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Template execution error: %v", err)
	}

	return buf.String()
}

func renderErrorPage(message string) string {
	tmpl, err := template.New("error").Parse(errorPageTemplate)
	if err != nil {
		return fmt.Sprintf("Template error: %v", err)
	}

	data := errorPageData{Message: message}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Template execution error: %v", err)
	}

	return buf.String()
}

func main() {
	lambda.Start(handler)
}
