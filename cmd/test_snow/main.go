package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tiaaburton/Data-Recon-Agent/pkg/envutil"
)

func main() {
	envutil.LoadEnvFile(".env")
	envutil.LoadEnvFile(".env.local")

	snowURL := os.Getenv("SERVICENOW_INSTANCE_URL")
	snowUser := os.Getenv("SERVICENOW_USERNAME")
	snowPass := os.Getenv("SERVICENOW_PASSWORD")

	baseURL := strings.TrimRight(snowURL, "/")
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(snowUser+":"+snowPass))

	fmt.Printf("=== ServiceNow Multi-Endpoint REST Audit ===\n")
	fmt.Printf("Target: %s | User: %s | Pass: %s\n\n", baseURL, snowUser, snowPass)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	endpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"Table API (Default)", "GET", "/api/now/table/incident?sysparm_limit=1"},
		{"Table API v1", "GET", "/api/now/v1/table/incident?sysparm_limit=1"},
		{"Table API v2", "GET", "/api/now/v2/table/incident?sysparm_limit=1"},
		{"Stats API", "GET", "/api/now/stats/incident?sysparm_count=true"},
		{"User Table", "GET", "/api/now/table/sys_user?sysparm_query=user_name=" + snowUser},
		{"Task Table", "GET", "/api/now/table/task?sysparm_limit=1"},
		{"Attachment API", "GET", "/api/now/attachment?sysparm_limit=1"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, baseURL+ep.path, nil)
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[%s] %s -> ERROR: %v\n", ep.method, ep.path, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Printf("[%s] %-35s -> Status: %d %s | Body: %s\n", ep.method, ep.name, resp.StatusCode, resp.Status, strings.TrimSpace(string(body[:min(len(body), 100)])))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
