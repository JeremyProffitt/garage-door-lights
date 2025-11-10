package main

import (
	"context"
	"fmt"
	"log"

	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"golang.org/x/crypto/bcrypt"
)

const (
	adminUser     = "Jeremy"
	adminPassword = "Ninja4President"
	stackName     = "candle-lights"
	region        = "us-east-1"
	bcryptCost    = 14 // Must match backend cost factor
)

// User represents a user in the system
type User struct {
	Username         string    `dynamodbav:"username"`
	PasswordHash     string    `dynamodbav:"passwordHash"`
	ParticleToken    string    `dynamodbav:"particleToken"`
	ParticleUsername string    `dynamodbav:"particleUsername"`
	CreatedAt        time.Time `dynamodbav:"createdAt"`
	UpdatedAt        time.Time `dynamodbav:"updatedAt"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

func run() error {
	ctx := context.Background()

	fmt.Println("Creating/updating admin user in DynamoDB...")

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Get table name from CloudFormation
	tableName, err := getTableNameFromStack(ctx, cfg, stackName)
	if err != nil {
		return err
	}

	fmt.Printf("Using DynamoDB table: %s\n", tableName)

	// Hash password using bcrypt cost=14 to match backend
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	fmt.Printf("Generated bcrypt hash: %s\n", string(passwordHash))

	// Create DynamoDB client
	dbClient := dynamodb.NewFromConfig(cfg)

	// Check if user already exists
	existingUser, err := getExistingUser(ctx, dbClient, tableName, adminUser)
	if err != nil {
		fmt.Printf("Warning: Could not check for existing user: %v\n", err)
	}

	// Create or update admin user
	timestamp := time.Now().UTC()
	user := User{
		Username:     adminUser,
		PasswordHash: string(passwordHash),
		UpdatedAt:    timestamp,
	}

	if existingUser != nil {
		fmt.Printf("Admin user '%s' already exists, updating...\n", adminUser)
		// Preserve existing fields
		user.ParticleToken = existingUser.ParticleToken
		user.ParticleUsername = existingUser.ParticleUsername
		user.CreatedAt = existingUser.CreatedAt
	} else {
		fmt.Printf("Creating new admin user '%s'...\n", adminUser)
		user.ParticleToken = ""
		user.ParticleUsername = ""
		user.CreatedAt = timestamp
	}

	// Convert to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	// Put item to DynamoDB
	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	action := "created"
	if existingUser != nil {
		action = "updated"
	}

	fmt.Printf("✓ Admin user '%s' %s successfully\n\n", adminUser, action)
	fmt.Println("Login credentials:")
	fmt.Printf("  Username: %s\n", adminUser)
	fmt.Printf("  Password: %s\n", adminPassword)

	return nil
}

// getTableNameFromStack retrieves the Users table name from CloudFormation stack outputs
func getTableNameFromStack(ctx context.Context, cfg aws.Config, stackName string) (string, error) {
	cfClient := cloudformation.NewFromConfig(cfg)

	output, err := cfClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return "", fmt.Errorf("stack '%s' not found", stackName)
	}

	for _, stackOutput := range output.Stacks[0].Outputs {
		if stackOutput.OutputKey != nil && *stackOutput.OutputKey == "UsersTableName" {
			if stackOutput.OutputValue != nil {
				return *stackOutput.OutputValue, nil
			}
		}
	}

	return "", fmt.Errorf("could not find UsersTableName in CloudFormation outputs")
}

// getExistingUser retrieves an existing user from DynamoDB if it exists
func getExistingUser(ctx context.Context, dbClient *dynamodb.Client, tableName, username string) (*User, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"username": username,
	})
	if err != nil {
		return nil, err
	}

	result, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var user User
	err = attributevalue.UnmarshalMap(result.Item, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
