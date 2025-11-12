package shared

import (
    "context"
    "log"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client

// InitDynamoDB initializes the DynamoDB client
func InitDynamoDB() (*dynamodb.Client, error) {
    if dynamoClient != nil {
        log.Println("[DB] Using cached DynamoDB client")
        return dynamoClient, nil
    }

    log.Println("[DB] Initializing new DynamoDB client")
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        log.Printf("[DB] ERROR: Failed to load AWS config: %v", err)
        return nil, err
    }

    dynamoClient = dynamodb.NewFromConfig(cfg)
    log.Println("[DB] DynamoDB client initialized successfully")
    return dynamoClient, nil
}

// GetItem retrieves an item from DynamoDB
func GetItem(ctx context.Context, tableName string, key map[string]types.AttributeValue, result interface{}) error {
    log.Printf("[DB] GetItem: table=%s, key=%v", tableName, key)

    client, err := InitDynamoDB()
    if err != nil {
        log.Printf("[DB] GetItem ERROR: Failed to initialize DynamoDB: %v", err)
        return err
    }

    output, err := client.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: &tableName,
        Key:       key,
    })
    if err != nil {
        log.Printf("[DB] GetItem ERROR: Failed to get item from %s: %v", tableName, err)
        return err
    }

    if output.Item == nil {
        log.Printf("[DB] GetItem: No item found in %s", tableName)
        return nil
    }

    err = attributevalue.UnmarshalMap(output.Item, result)
    if err != nil {
        log.Printf("[DB] GetItem ERROR: Failed to unmarshal item from %s: %v", tableName, err)
        return err
    }

    log.Printf("[DB] GetItem: Successfully retrieved item from %s", tableName)
    return nil
}

// PutItem puts an item into DynamoDB
func PutItem(ctx context.Context, tableName string, item interface{}) error {
    log.Printf("[DB] PutItem: table=%s, item type=%T", tableName, item)
    log.Printf("[DB] PutItem: item value=%+v", item)

    client, err := InitDynamoDB()
    if err != nil {
        log.Printf("[DB] PutItem ERROR: Failed to initialize DynamoDB: %v", err)
        return err
    }

    av, err := attributevalue.MarshalMap(item)
    if err != nil {
        log.Printf("[DB] PutItem ERROR: Failed to marshal item for %s: %v", tableName, err)
        return err
    }

    // Log the marshaled attributes to see what's being sent to DynamoDB
    log.Printf("[DB] PutItem: marshaled AttributeValues count=%d", len(av))
    for key, val := range av {
        log.Printf("[DB] PutItem: marshaled field %s type=%T", key, val)
    }

    _, err = client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: &tableName,
        Item:      av,
    })

    if err != nil {
        log.Printf("[DB] PutItem ERROR: Failed to put item into %s: %v", tableName, err)
        return err
    }

    log.Printf("[DB] PutItem: Successfully put item into %s", tableName)
    return nil
}

// DeleteItem deletes an item from DynamoDB
func DeleteItem(ctx context.Context, tableName string, key map[string]types.AttributeValue) error {
    log.Printf("[DB] DeleteItem: table=%s, key=%v", tableName, key)

    client, err := InitDynamoDB()
    if err != nil {
        log.Printf("[DB] DeleteItem ERROR: Failed to initialize DynamoDB: %v", err)
        return err
    }

    _, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
        TableName: &tableName,
        Key:       key,
    })

    if err != nil {
        log.Printf("[DB] DeleteItem ERROR: Failed to delete item from %s: %v", tableName, err)
        return err
    }

    log.Printf("[DB] DeleteItem: Successfully deleted item from %s", tableName)
    return nil
}

// Query performs a query on DynamoDB
func Query(ctx context.Context, tableName string, indexName *string, keyCondition string,
    expressionValues map[string]types.AttributeValue, results interface{}) error {
    indexInfo := "none"
    if indexName != nil {
        indexInfo = *indexName
    }
    log.Printf("[DB] Query: table=%s, index=%s, condition=%s", tableName, indexInfo, keyCondition)

    client, err := InitDynamoDB()
    if err != nil {
        log.Printf("[DB] Query ERROR: Failed to initialize DynamoDB: %v", err)
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
        log.Printf("[DB] Query ERROR: Failed to query %s: %v", tableName, err)
        return err
    }

    err = attributevalue.UnmarshalListOfMaps(output.Items, results)
    if err != nil {
        log.Printf("[DB] Query ERROR: Failed to unmarshal results from %s: %v", tableName, err)
        return err
    }

    log.Printf("[DB] Query: Successfully queried %s, found %d items", tableName, len(output.Items))
    return nil
}

// Scan performs a scan on DynamoDB
func Scan(ctx context.Context, tableName string, results interface{}) error {
    log.Printf("[DB] Scan: table=%s", tableName)

    client, err := InitDynamoDB()
    if err != nil {
        log.Printf("[DB] Scan ERROR: Failed to initialize DynamoDB: %v", err)
        return err
    }

    output, err := client.Scan(ctx, &dynamodb.ScanInput{
        TableName: &tableName,
    })
    if err != nil {
        log.Printf("[DB] Scan ERROR: Failed to scan %s: %v", tableName, err)
        return err
    }

    err = attributevalue.UnmarshalListOfMaps(output.Items, results)
    if err != nil {
        log.Printf("[DB] Scan ERROR: Failed to unmarshal results from %s: %v", tableName, err)
        return err
    }

    log.Printf("[DB] Scan: Successfully scanned %s, found %d items", tableName, len(output.Items))
    return nil
}
