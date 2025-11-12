package shared

import (
    "context"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client

// InitDynamoDB initializes the DynamoDB client
func InitDynamoDB() (*dynamodb.Client, error) {
    if dynamoClient != nil {
        return dynamoClient, nil
    }

    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        return nil, err
    }

    dynamoClient = dynamodb.NewFromConfig(cfg)
    return dynamoClient, nil
}

// GetItem retrieves an item from DynamoDB
func GetItem(ctx context.Context, tableName string, key map[string]types.AttributeValue, result interface{}) error {
    client, err := InitDynamoDB()
    if err != nil {
        return err
    }

    output, err := client.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: &tableName,
        Key:       key,
    })
    if err != nil {
        return err
    }

    if output.Item == nil {
        return nil
    }

    return attributevalue.UnmarshalMap(output.Item, result)
}

// PutItem puts an item into DynamoDB
func PutItem(ctx context.Context, tableName string, item interface{}) error {
    client, err := InitDynamoDB()
    if err != nil {
        return err
    }

    av, err := attributevalue.MarshalMap(item)
    if err != nil {
        return err
    }

    _, err = client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: &tableName,
        Item:      av,
    })

    return err
}

// DeleteItem deletes an item from DynamoDB
func DeleteItem(ctx context.Context, tableName string, key map[string]types.AttributeValue) error {
    client, err := InitDynamoDB()
    if err != nil {
        return err
    }

    _, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
        TableName: &tableName,
        Key:       key,
    })

    return err
}

// Query performs a query on DynamoDB
func Query(ctx context.Context, tableName string, indexName *string, keyCondition string,
    expressionValues map[string]types.AttributeValue, results interface{}) error {
    client, err := InitDynamoDB()
    if err != nil {
        return err
    }

    input := &dynamodb.QueryInput{
        TableName:                 &tableName,
        KeyConditionExpression:    &keyCondition,
        ExpressionAttributeValues: expressionValues,
    }

    if indexName != nil {
        input.IndexName = indexName
    }

    output, err := client.Query(ctx, input)
    if err != nil {
        return err
    }

    return attributevalue.UnmarshalListOfMaps(output.Items, results)
}

// Scan performs a scan on DynamoDB
func Scan(ctx context.Context, tableName string, results interface{}) error {
    client, err := InitDynamoDB()
    if err != nil {
        return err
    }

    output, err := client.Scan(ctx, &dynamodb.ScanInput{
        TableName: &tableName,
    })
    if err != nil {
        return err
    }

    return attributevalue.UnmarshalListOfMaps(output.Items, results)
}
