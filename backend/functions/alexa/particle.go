package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const particleAPIBase = "https://api.particle.io/v1"

// callParticleFunction calls a Particle cloud function on a device
func callParticleFunction(deviceID, functionName, argument, token string) error {
	url := fmt.Sprintf("%s/devices/%s/%s", particleAPIBase, deviceID, functionName)

	log.Printf("Calling Particle function: %s on device %s with arg: %s", functionName, deviceID, argument)

	data := map[string]string{
		"arg": argument,
	}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Particle API error (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("Particle API error: %s", string(body))
	}

	log.Printf("Particle function call successful")
	return nil
}
