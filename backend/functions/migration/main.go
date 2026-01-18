package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"candle-lights/backend/shared"
)

var (
	patternsTable      = os.Getenv("PATTERNS_TABLE")
	conversationsTable = os.Getenv("CONVERSATIONS_TABLE")
	ddbClient          *dynamodb.Client
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	ddbClient = dynamodb.NewFromConfig(cfg)
}

// MigrationRequest contains migration parameters
type MigrationRequest struct {
	DryRun       bool `json:"dryRun"`       // If true, don't write changes
	MaxItems     int  `json:"maxItems"`     // Max items to migrate (0 = all)
	MigrateConvs bool `json:"migrateConvs"` // Also migrate conversations
}

// MigrationResult contains migration statistics
type MigrationResult struct {
	PatternsMigrated     int      `json:"patternsMigrated"`
	PatternsSkipped      int      `json:"patternsSkipped"`
	PatternsFailed       int      `json:"patternsFailed"`
	ConvsMigrated        int      `json:"convsMigrated"`
	ConvsSkipped         int      `json:"convsSkipped"`
	ConvsFailed          int      `json:"convsFailed"`
	DryRun               bool     `json:"dryRun"`
	Errors               []string `json:"errors,omitempty"`
	MigratedPatternNames []string `json:"migratedPatternNames,omitempty"`
}

func handler(ctx context.Context, request MigrationRequest) (MigrationResult, error) {
	log.Printf("=== Migration Handler Called ===")
	log.Printf("DryRun: %v, MaxItems: %d, MigrateConvs: %v", request.DryRun, request.MaxItems, request.MigrateConvs)

	result := MigrationResult{
		DryRun: request.DryRun,
	}

	// Migrate patterns
	if err := migratePatterns(ctx, &request, &result); err != nil {
		log.Printf("Pattern migration error: %v", err)
		result.Errors = append(result.Errors, "Pattern migration failed: "+err.Error())
	}

	// Migrate conversations if requested
	if request.MigrateConvs {
		if err := migrateConversations(ctx, &request, &result); err != nil {
			log.Printf("Conversation migration error: %v", err)
			result.Errors = append(result.Errors, "Conversation migration failed: "+err.Error())
		}
	}

	log.Printf("=== Migration Complete ===")
	log.Printf("Patterns: migrated=%d, skipped=%d, failed=%d",
		result.PatternsMigrated, result.PatternsSkipped, result.PatternsFailed)
	if request.MigrateConvs {
		log.Printf("Conversations: migrated=%d, skipped=%d, failed=%d",
			result.ConvsMigrated, result.ConvsSkipped, result.ConvsFailed)
	}

	return result, nil
}

func migratePatterns(ctx context.Context, request *MigrationRequest, result *MigrationResult) error {
	// Scan all patterns
	input := &dynamodb.ScanInput{
		TableName: aws.String(patternsTable),
	}

	paginator := dynamodb.NewScanPaginator(ddbClient, input)
	count := 0

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, item := range page.Items {
			if request.MaxItems > 0 && count >= request.MaxItems {
				log.Printf("Reached max items limit: %d", request.MaxItems)
				return nil
			}

			var pattern shared.Pattern
			if err := attributevalue.UnmarshalMap(item, &pattern); err != nil {
				log.Printf("Failed to unmarshal pattern: %v", err)
				result.PatternsFailed++
				continue
			}

			// Skip if already WLED format
			if pattern.FormatVersion == shared.FormatVersionWLED {
				log.Printf("Skipping pattern %s (%s) - already WLED format", pattern.PatternID, pattern.Name)
				result.PatternsSkipped++
				continue
			}

			// Skip if no GlowBlaster data
			if pattern.LCLSpec == "" && pattern.IntentLayer == "" && len(pattern.Bytecode) == 0 {
				log.Printf("Skipping pattern %s (%s) - no LCL data", pattern.PatternID, pattern.Name)
				result.PatternsSkipped++
				continue
			}

			// Migrate the pattern
			if err := migratePattern(ctx, &pattern, request.DryRun); err != nil {
				log.Printf("Failed to migrate pattern %s (%s): %v", pattern.PatternID, pattern.Name, err)
				result.PatternsFailed++
				result.Errors = append(result.Errors, pattern.PatternID+": "+err.Error())
				continue
			}

			log.Printf("Migrated pattern %s (%s)", pattern.PatternID, pattern.Name)
			result.PatternsMigrated++
			result.MigratedPatternNames = append(result.MigratedPatternNames, pattern.Name)
			count++
		}
	}

	return nil
}

func migratePattern(ctx context.Context, pattern *shared.Pattern, dryRun bool) error {
	// Determine LED count (default 8)
	ledCount := 8

	// Parse and convert LCL to WLED
	var wledState *shared.WLEDState
	var err error

	if pattern.LCLSpec != "" {
		// Try to parse LCL spec
		wledState, err = convertLCLSpecToWLED(pattern.LCLSpec, ledCount)
		if err != nil {
			log.Printf("Failed to convert LCL spec: %v, trying bytecode", err)
		}
	}

	if wledState == nil && len(pattern.Bytecode) > 0 {
		// Try to convert from bytecode
		wledState, err = convertBytecodeDToWLED(pattern.Bytecode, ledCount)
		if err != nil {
			return err
		}
	}

	if wledState == nil {
		// Create default WLED state based on pattern type
		wledState = createDefaultWLEDFromPattern(pattern, ledCount)
	}

	// Compile to binary
	wledBinary, err := shared.CompileWLEDToBinary(wledState)
	if err != nil {
		return err
	}

	// Marshal WLED state to JSON
	wledJSON, err := json.Marshal(wledState)
	if err != nil {
		return err
	}

	if dryRun {
		log.Printf("  [DRY RUN] Would update pattern with WLED: %s", string(wledJSON))
		return nil
	}

	// Update pattern in DynamoDB
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(patternsTable),
		Key: map[string]types.AttributeValue{
			"patternId": &types.AttributeValueMemberS{Value: pattern.PatternID},
		},
		UpdateExpression: aws.String("SET wledState = :wled, wledBinary = :bin, formatVersion = :v"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":wled": &types.AttributeValueMemberS{Value: string(wledJSON)},
			":bin":  &types.AttributeValueMemberB{Value: wledBinary},
			":v":    &types.AttributeValueMemberN{Value: "2"},
		},
	}

	_, err = ddbClient.UpdateItem(ctx, updateInput)
	return err
}

func convertLCLSpecToWLED(lclSpec string, ledCount int) (*shared.WLEDState, error) {
	// Parse LCL YAML spec
	spec, err := shared.ParseIntentYAML(lclSpec)
	if err != nil {
		return nil, err
	}

	// Use the shared conversion function
	return shared.ConvertLCLToWLED(spec, ledCount)
}

func convertBytecodeDToWLED(bytecode []byte, ledCount int) (*shared.WLEDState, error) {
	// Check for LCL format
	if len(bytecode) < 8 || string(bytecode[0:3]) != "LCL" {
		return nil, nil
	}

	// Extract basic info from bytecode (simplified)
	// Bytecode format: LCL + version(1) + length(2) + checksum(1) + flags(1) + effect(1) + brightness(1) + speed(1) + ...
	effectByte := bytecode[8]
	brightness := bytecode[9]
	speed := bytecode[10]

	// Map LCL bytecode effect ID to WLED effect ID
	// LCL effect IDs: 0x01=solid, 0x02=pulse, 0x04=sparkle, 0x07=fire, 0x08=candle, 0x09=wave, 0x0A=rainbow, 0x0B=scanner
	var wledEffectID int
	switch effectByte {
	case 0x01:
		wledEffectID = shared.WLEDFXSolid
	case 0x02:
		wledEffectID = shared.WLEDFXBreathe
	case 0x04:
		wledEffectID = shared.WLEDFXSparkle
	case 0x07:
		wledEffectID = shared.WLEDFXFire2012
	case 0x08:
		wledEffectID = shared.WLEDFXCandle
	case 0x09:
		wledEffectID = shared.WLEDFXColorwaves
	case 0x0A:
		wledEffectID = shared.WLEDFXRainbow
	case 0x0B:
		wledEffectID = shared.WLEDFXScanner
	default:
		wledEffectID = shared.WLEDFXSolid
	}

	// Extract colors if present (simplified - full extraction would need proper bytecode parsing)
	primaryR := uint8(255)
	primaryG := uint8(147)
	primaryB := uint8(41)

	if len(bytecode) >= 27 {
		primaryR = bytecode[24]
		primaryG = bytecode[25]
		primaryB = bytecode[26]
	}

	return &shared.WLEDState{
		On:         true,
		Brightness: int(brightness),
		Segments: []shared.WLEDSegment{
			{
				ID:        0,
				Start:     0,
				Stop:      ledCount,
				EffectID:  wledEffectID,
				Speed:     int(speed),
				Intensity: 128,
				Colors: [][]int{
					{int(primaryR), int(primaryG), int(primaryB)},
				},
				On: true,
			},
		},
	}, nil
}

func createDefaultWLEDFromPattern(pattern *shared.Pattern, ledCount int) *shared.WLEDState {
	// Map pattern type to WLED effect
	effectID := 0 // Default solid
	switch pattern.Type {
	case shared.PatternCandle:
		effectID = shared.WLEDFXCandle
	case shared.PatternSolid:
		effectID = shared.WLEDFXSolid
	case shared.PatternPulse:
		effectID = shared.WLEDFXBreathe
	case shared.PatternWave:
		effectID = shared.WLEDFXColorwaves
	case shared.PatternRainbow:
		effectID = shared.WLEDFXRainbow
	case shared.PatternFire:
		effectID = shared.WLEDFXFire2012
	}

	// Get color
	r, g, b := pattern.Red, pattern.Green, pattern.Blue
	if r == 0 && g == 0 && b == 0 {
		r, g, b = 255, 147, 41 // Default warm color
	}

	brightness := pattern.Brightness
	if brightness == 0 {
		brightness = 200
	}

	speed := pattern.Speed
	if speed == 0 {
		speed = 128
	}

	return &shared.WLEDState{
		On:         true,
		Brightness: brightness,
		Segments: []shared.WLEDSegment{
			{
				ID:        0,
				Start:     0,
				Stop:      ledCount,
				EffectID:  effectID,
				Speed:     speed,
				Intensity: 128,
				Colors: [][]int{
					{r, g, b},
				},
				On: true,
			},
		},
	}
}

func migrateConversations(ctx context.Context, request *MigrationRequest, result *MigrationResult) error {
	// Scan all conversations
	input := &dynamodb.ScanInput{
		TableName: aws.String(conversationsTable),
	}

	paginator := dynamodb.NewScanPaginator(ddbClient, input)
	count := 0

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, item := range page.Items {
			if request.MaxItems > 0 && count >= request.MaxItems {
				return nil
			}

			var conv shared.Conversation
			if err := attributevalue.UnmarshalMap(item, &conv); err != nil {
				log.Printf("Failed to unmarshal conversation: %v", err)
				result.ConvsFailed++
				continue
			}

			// Skip if already has WLED data
			if conv.CurrentWLED != "" {
				result.ConvsSkipped++
				continue
			}

			// Skip if no LCL data
			if conv.CurrentLCL == "" {
				result.ConvsSkipped++
				continue
			}

			// Migrate conversation
			if err := migrateConversation(ctx, &conv, request.DryRun); err != nil {
				log.Printf("Failed to migrate conversation %s: %v", conv.ConversationID, err)
				result.ConvsFailed++
				continue
			}

			result.ConvsMigrated++
			count++
		}
	}

	return nil
}

func migrateConversation(ctx context.Context, conv *shared.Conversation, dryRun bool) error {
	// Convert LCL to WLED
	wledState, err := convertLCLSpecToWLED(conv.CurrentLCL, 8)
	if err != nil {
		return err
	}

	// Compile to binary
	wledBinary, err := shared.CompileWLEDToBinary(wledState)
	if err != nil {
		return err
	}

	// Marshal WLED state to JSON
	wledJSON, err := json.Marshal(wledState)
	if err != nil {
		return err
	}

	if dryRun {
		log.Printf("  [DRY RUN] Would update conversation with WLED")
		return nil
	}

	// Update conversation in DynamoDB
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(conversationsTable),
		Key: map[string]types.AttributeValue{
			"conversationId": &types.AttributeValueMemberS{Value: conv.ConversationID},
		},
		UpdateExpression: aws.String("SET currentWled = :wled, currentWledBin = :bin"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":wled": &types.AttributeValueMemberS{Value: string(wledJSON)},
			":bin":  &types.AttributeValueMemberB{Value: wledBinary},
		},
	}

	_, err = ddbClient.UpdateItem(ctx, updateInput)
	return err
}

func main() {
	lambda.Start(handler)
}
