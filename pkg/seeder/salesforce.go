package seeder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

// SalesforceSeeder manages authentication and batch creation of Opportunity records.
type SalesforceSeeder struct {
	InstanceURL string
	AccessToken string
	HTTPClient  *http.Client
}

// NewSalesforceSeeder creates an authenticated Salesforce seeder.
func NewSalesforceSeeder(instanceURL, accessToken string) *SalesforceSeeder {
	return &SalesforceSeeder{
		InstanceURL: instanceURL,
		AccessToken: accessToken,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// AuthenticateSalesforce performs OAuth2 password authentication against Salesforce.
func AuthenticateSalesforce(instanceURL, clientID, clientSecret, username, password string) (string, error) {
	tokenURL := fmt.Sprintf("%s/services/oauth2/token", instanceURL)
	formData := url.Values{
		"grant_type":    {"password"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"username":      {username},
		"password":      {password},
	}

	// #nosec G107 -- Dynamic OAuth2 endpoint for configurable Salesforce instance
	resp, err := http.PostForm(tokenURL, formData)
	if err != nil {
		return "", fmt.Errorf("failed to reach salesforce oauth endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("salesforce oauth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		InstanceURL string `json:"instance_url"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse oauth response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// SeedOpportunity loads a single Opportunity into Salesforce.
func (s *SalesforceSeeder) SeedOpportunity(ctx context.Context, opp schemas.SalesforceOpportunitySeed) (string, error) {
	endpoint := fmt.Sprintf("%s/services/data/v60.0/sobjects/Opportunity", s.InstanceURL)
	payload, err := json.Marshal(opp)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("salesforce http error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("salesforce insert failed (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		ID      string   `json:"id"`
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return res.ID, nil
}
