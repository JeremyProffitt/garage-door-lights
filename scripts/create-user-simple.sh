#!/bin/bash

# Simplified user creation script using Python for password hashing
# Usage: ./create-user-simple.sh <stack-name> <username> <password> [particle-token]

set -e

if [ $# -lt 3 ]; then
    echo "Usage: $0 <stack-name> <username> <password> [particle-token]"
    echo "Example: $0 candle-lights-prod john mypassword abc123"
    exit 1
fi

STACK_NAME=$1
USERNAME=$2
PASSWORD=$3
PARTICLE_TOKEN=${4:-""}

# Get table name from CloudFormation stack
TABLE_NAME=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --query "Stacks[0].Outputs[?OutputKey=='UsersTableName'].OutputValue" \
    --output text)

if [ -z "$TABLE_NAME" ]; then
    echo "Error: Could not find UsersTable in stack $STACK_NAME"
    exit 1
fi

echo "Creating user in table: $TABLE_NAME"

# Generate password hash using Python
HASH=$(python3 -c "import bcrypt; print(bcrypt.hashpw(b'$PASSWORD', bcrypt.gensalt(14)).decode())")

# Get current timestamp
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Create user item
aws dynamodb put-item \
    --table-name "$TABLE_NAME" \
    --item "{
        \"username\": {\"S\": \"$USERNAME\"},
        \"passwordHash\": {\"S\": \"$HASH\"},
        \"particleToken\": {\"S\": \"$PARTICLE_TOKEN\"},
        \"particleUsername\": {\"S\": \"\"},
        \"createdAt\": {\"S\": \"$TIMESTAMP\"},
        \"updatedAt\": {\"S\": \"$TIMESTAMP\"}
    }"

echo "User created successfully!"
echo "Username: $USERNAME"
