package seeder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/schemas"
)

// ServiceNowSeeder manages Table API interaction for Incident dispute records.
type ServiceNowSeeder struct {
	InstanceURL string
	Username    string
	Password    string
	HTTPClient  *http.Client
}

// NewServiceNowSeeder creates a ServiceNow seeder.
func NewServiceNowSeeder(instanceURL, username, password string) *ServiceNowSeeder {
	return &ServiceNowSeeder{
		InstanceURL: instanceURL,
		Username:    username,
		Password:    password,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SeedIncident loads a single dispute record into the ServiceNow incident table.
func (s *ServiceNowSeeder) SeedIncident(ctx context.Context, inc schemas.ServiceNowIncidentSeed) (string, error) {
	endpoint := fmt.Sprintf("%s/api/now/table/incident", s.InstanceURL)
	payload, err := json.Marshal(inc)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(s.Username, s.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("servicenow http error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("servicenow insert failed (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Result struct {
			SysID  string `json:"sys_id"`
			Number string `json:"number"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse servicenow response: %w", err)
	}

	return res.Result.Number, nil
}
