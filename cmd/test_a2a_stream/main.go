package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

func getAccessToken(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth print-access-token failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	token, err := getAccessToken(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain GCP access token: %v", err)
	}

	const (
		projectID = "tias-demos"
		agentID   = "11103644724060655448"
	)

	endpoints := []string{
		fmt.Sprintf("https://us-discoveryengine.googleapis.com/v1/projects/14200540645/locations/us/collections/default_collection/engines/gemini-app/assistants/default_assistant/agents/%s/a2a/v1/message:stream", agentID),
		fmt.Sprintf("https://us-discoveryengine.googleapis.com/v1alpha/projects/14200540645/locations/us/collections/default_collection/engines/gemini-app/assistants/default_assistant/agents/%s/a2a/v1/message:stream", agentID),
	}

	payloads := []map[string]any{
		{
			"message": map[string]any{
				"role": "ROLE_USER",
				"content": []any{
					map[string]any{
						"text": "Reconcile contract CTR-2026-451",
					},
				},
			},
		},
		{
			"message": map[string]any{
				"role": "ROLE_USER",
				"parts": []any{
					map[string]any{
						"text": "Reconcile contract CTR-2026-451",
					},
				},
			},
		},
		{
			"message": map[string]any{
				"role": "USER",
				"content": []any{
					map[string]any{
						"text": "Reconcile contract CTR-2026-451",
					},
				},
			},
		},
		{
			"message": map[string]any{
				"role": "ROLE_USER",
				"text": "Reconcile contract CTR-2026-451",
			},
		},
	}

	client := &http.Client{Timeout: 90 * time.Second}

	for pIdx, reqBody := range payloads {
		bodyBytes, _ := json.Marshal(reqBody)
		fmt.Printf("\n=======================================================\n")
		fmt.Printf("🚀 Payload Shape %d: %s\n", pIdx+1, string(bodyBytes))
		fmt.Printf("=======================================================\n")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoints[0], bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Goog-User-Project", projectID)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("HTTP request error: %v", err)
			continue
		}

		fmt.Printf("HTTP Status: %s\n", resp.Status)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("Response Error Body:\n%s\n", string(body))
			continue
		}

		reader := bufio.NewReader(resp.Body)
		chunkIndex := 0
		var downloadURIs []string
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				chunkIndex++
				trimmed := strings.TrimSpace(string(line))
				var parsed any
				if jsonErr := json.Unmarshal([]byte(trimmed), &parsed); jsonErr == nil {
					pretty, _ := json.MarshalIndent(parsed, "", "  ")
					fmt.Printf("\n--- [CHUNK %d] ---\n%s\n", chunkIndex, string(pretty))

					// Check for downloadFile URIs
					if mList, ok := parsed.([]any); ok {
						for _, it := range mList {
							if itMap, isM := it.(map[string]any); isM {
								if msgMap, hasMsg := itMap["message"].(map[string]any); hasMsg {
									if contentList, hasC := msgMap["content"].([]any); hasC {
										for _, c := range contentList {
											if cMap, isC := c.(map[string]any); isC {
												if fileMap, hasF := cMap["file"].(map[string]any); hasF {
													if uri, hasUri := fileMap["fileWithUri"].(string); hasUri {
														downloadURIs = append(downloadURIs, uri)
													}
												}
											}
										}
									}
								}
							}
						}
					}
				} else {
					fmt.Printf("\n--- [CHUNK %d] ---\n%s\n", chunkIndex, trimmed)
				}
			}
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Stream read error: %v\n", err)
				}
				break
			}
		}
		resp.Body.Close()

		if len(downloadURIs) > 0 {
			fmt.Printf("\n=======================================================\n")
			fmt.Printf("📥 Inspecting %d Downloaded A2UI Artifact(s):\n", len(downloadURIs))
			fmt.Printf("=======================================================\n")
			for dIdx, uri := range downloadURIs {
				dReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
				dReq.Header.Set("Authorization", "Bearer "+token)
				dReq.Header.Set("X-Goog-User-Project", projectID)
				dResp, dErr := client.Do(dReq)
				if dErr == nil {
					dBody, _ := io.ReadAll(dResp.Body)
					dResp.Body.Close()
					var fileParsed any
					if jErr := json.Unmarshal(dBody, &fileParsed); jErr == nil {
						pJSON, _ := json.MarshalIndent(fileParsed, "", "  ")
						fmt.Printf("\n[Artifact %d Content (%s)]:\n%s\n", dIdx+1, uri, string(pJSON))
					} else {
						fmt.Printf("\n[Artifact %d Content (%s)]:\n%s\n", dIdx+1, uri, string(dBody))
					}
				} else {
					fmt.Printf("Failed to download %s: %v\n", uri, dErr)
				}
			}
		}
		break // If success, stop
	}
}
