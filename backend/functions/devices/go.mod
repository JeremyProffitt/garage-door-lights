module candle-lights/backend/functions/devices

go 1.21

require (
    candle-lights/backend/shared v0.0.0
    github.com/aws/aws-lambda-go v1.41.0
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/config v1.26.1
    github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.12.13
    github.com/aws/aws-sdk-go-v2/service/dynamodb v1.26.7
    github.com/google/uuid v1.5.0
)

replace candle-lights/backend/shared => ./shared
