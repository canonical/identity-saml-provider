package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	serverURL    string
	entityID     string
	acsURL       string
	acsBinding   string
	outputFormat string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "service-provider-admin",
		Short: "CLI tool to manage SAML service providers",
		Long:  "A command-line tool to add and manage SAML service providers via the Identity SAML Provider admin API",
	}

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new SAML service provider",
		Long:  "Register a new SAML service provider with the Identity SAML Provider",
		RunE:  runAdd,
	}

	// Add flags
	addCmd.Flags().StringVar(&serverURL, "server", "http://localhost:8082", "Base URL of the Identity SAML Provider server")
	addCmd.Flags().StringVarP(&entityID, "entity-id", "e", "", "Entity ID (unique identifier) of the service provider (required, must be a valid URL)")
	addCmd.Flags().StringVarP(&acsURL, "acs-url", "a", "", "Assertion Consumer Service (ACS) URL (required, must be a valid URL)")
	addCmd.Flags().StringVarP(&acsBinding, "acs-binding", "b", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST", "ACS binding type (optional, defaults to HTTP-POST)")
	addCmd.Flags().StringVar(&outputFormat, "output", "human", "Output format: 'human' for human-readable or 'json' for JSON")

	// Mark required flags
	addCmd.MarkFlagRequired("entity-id")
	addCmd.MarkFlagRequired("acs-url")

	rootCmd.AddCommand(addCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Ensure server URL doesn't have trailing slash
	serverURL = strings.TrimSuffix(serverURL, "/")

	endpoint := serverURL + "/admin/service-providers"

	// Prepare request body
	requestBody := map[string]string{
		"entity_id":   entityID,
		"acs_url":     acsURL,
		"acs_binding": acsBinding,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code first
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response only on success
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("server returned success status but response was not valid JSON: %w", err)
	}

	// Print output based on format
	if outputFormat == "json" {
		// Build JSON response
		jsonOutput := map[string]interface{}{
			"success":    true,
			"entity_id":  entityID,
			"acs_url":    acsURL,
			"acs_binding": acsBinding,
			"response":   response,
		}
		jsonBytes, err := json.MarshalIndent(jsonOutput, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		// Human-readable output
		fmt.Printf("✓ Service provider registered successfully!\n")
		fmt.Printf("  Entity ID: %s\n", entityID)
		fmt.Printf("  ACS URL: %s\n", acsURL)
		fmt.Printf("  ACS Binding: %s\n", acsBinding)
	}

	return nil
}
