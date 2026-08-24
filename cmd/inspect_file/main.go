package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Token err: %v\n", err)
		return
	}
	token := strings.TrimSpace(string(out))

	uris := []string{
		"https://us-discoveryengine.googleapis.com/v1/projects/14200540645/locations/us/collections/default_collection/engines/gemini-app/sessions/4412493197898354816:downloadFile?file_id=dccdf8d1-b50d-43c8-9ed2-76d62b842945&alt=media",
		"https://us-discoveryengine.googleapis.com/v1/projects/14200540645/locations/us/collections/default_collection/engines/gemini-app/sessions/4412493197898354816:downloadFile?file_id=58c71fb7-227a-48ba-80d4-2fbc1888ac70&alt=media",
	}

	for i, u := range uris {
		req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Goog-User-Project", "tias-demos")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Err: %v\n", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("File %d (%s):\n%s\n\n", i+1, resp.Status, string(body))
	}
}
