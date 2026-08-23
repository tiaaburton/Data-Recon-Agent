// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package method

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/genai"
	"google.golang.org/protobuf/types/known/structpb"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"github.com/tiaaburton/Data-Recon-Agent/internal/agentengine/internal/helper"
	"github.com/tiaaburton/Data-Recon-Agent/internal/agentengine/internal/models"
	"github.com/tiaaburton/Data-Recon-Agent/pkg/a2ui"
)

type streamingAgentRunWithEventsHandler struct {
	config        *launcher.Config
	methodName    string
	apiMode       string
	agentEngineID string
}

// NewStreamingAgentRunWithEventsHandler creates a new handler for streaming_agent_run_with_events.
func NewStreamingAgentRunWithEventsHandler(config *launcher.Config, agentEngineID, methodName, apiMode string) *streamingAgentRunWithEventsHandler {
	return &streamingAgentRunWithEventsHandler{config: config, agentEngineID: agentEngineID, methodName: methodName, apiMode: apiMode}
}

// Handle generates stream of json-encoded responses based on the payload. Errors are also emitted as errors.
func (s *streamingAgentRunWithEventsHandler) Handle(ctx context.Context, rw http.ResponseWriter, payload []byte) error {
	streamErr := s.streamJSONL(ctx, rw, payload)
	// streamJSONL will return error only before streaming. In that case we can handle it with HTTP Status, which is done in upstream.
	if streamErr != nil {
		err := fmt.Errorf("s.streamJSONL() failed: %w", streamErr)
		return err
	}
	return nil
}

// streamJSONL streams a single line for each event or error.
func (s *streamingAgentRunWithEventsHandler) streamJSONL(ctx context.Context, rw http.ResponseWriter, payload []byte) error {
	var req models.StreamingAgentRunWithEventsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		err = fmt.Errorf("json.Unmarshal() failed for models.StreamingAgentRunWithEventsRequest: %w", err)
		log.Print(err.Error())
		return err
	}

	runReq, requestedSessionID, err := decodeStreamingAgentRunWithEventsRequest(&req)
	if err != nil {
		err = fmt.Errorf("decodeStreamingAgentRunWithEventsRequest() failed: %w", err)
		log.Print(err.Error())
		return err
	}
	if err := s.ensureBackendSession(ctx, runReq, requestedSessionID); err != nil {
		err = fmt.Errorf("s.ensureBackendSession() failed: %w", err)
		log.Print(err.Error())
		return err
	}

	// A first turn can arrive carrying files and no session_id at all -- the
	// runner would create the session later, after the uploads have already
	// been staged. Create it now instead, so they are staged in that session's
	// own directory rather than a fallback shared by every anonymous first turn
	// in the container.
	if runReq.SessionID == "" && len(runReq.Artifacts) > 0 && runReq.UserID != "" &&
		s.config != nil && s.config.SessionService != nil {
		createResp, err := s.config.SessionService.Create(ctx, &session.CreateRequest{
			AppName: s.agentEngineID,
			UserID:  runReq.UserID,
		})
		if err != nil || createResp == nil || createResp.Session == nil {
			// Not fatal: spillArtifact falls back to a unique directory.
			log.Printf("[ARTIFACT] could not create a session to stage uploads in: %v", err)
		} else {
			runReq.SessionID = createResp.Session.ID()
		}
	}

	// Fold uploaded files into the message. Deliberately after session
	// normalization: runReq.SessionID is only the backend ADK session id from
	// here on, and large uploads are staged in that session's own directory.
	// Doing this earlier would stage under a Discovery Engine resource name,
	// which is not the directory the tools are confined to.
	artifactsToParts(runReq.Artifacts, &runReq.Message, runReq.SessionID)

	events, err := s.run(ctx, runReq, &runReq.Message, s.config)
	if err != nil {
		err = fmt.Errorf("s.run() failed: %w", err)
		log.Print(err.Error())
		return err
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	// from this moment on we must not return error. Instead, it should be handled by using helper.EmitJSONError

	for event, err := range events {
		log.Printf("Processing event: %+v err: %+v\n", event, err)
		if err != nil {
			log.Printf("error in events: %v\n", err)
			e := helper.EmitJSONError(rw, err)
			if e != nil {
				e = fmt.Errorf("helper.EmitJSONError() failed: %w", e)
				log.Print(e.Error())
			}
			break
		}
		if event == nil {
			continue
		}
		if event.Content == nil && event.LLMResponse.Content == nil {
			continue
		}

		respObj := models.StreamingAgentRunWithEventsResponse{
			Events:    []*session.Event{event},
			SessionID: runReq.SessionID,
		}

		converted := helper.ConvertSnake(respObj)
		if m, ok := converted.(map[string]any); ok {
			hasFuncResp := false
			if event.Content != nil {
				for _, p := range event.Content.Parts {
					if p.FunctionResponse != nil {
						hasFuncResp = true
						break
					}
				}
			}

			// If this is a model turn, inject pending A2UI DataParts into events[0].content.parts
			if !hasFuncResp {
				if pending := a2ui.PopPendingA2UIMessages(runReq.SessionID); len(pending) > 0 {
					if evs, ok := m["events"].([]any); ok && len(evs) > 0 {
						if ev0, ok := evs[0].(map[string]any); ok {
							dataParts := make([]any, 0, len(pending))
							for _, msg := range pending {
								dataParts = append(dataParts, a2ui.BuildA2UIDataPart(msg))
							}

							var targetContent map[string]any
							if content, ok := ev0["content"].(map[string]any); ok && content != nil {
								targetContent = content
							} else if llmResp, ok := ev0["llm_response"].(map[string]any); ok && llmResp != nil {
								if content, ok := llmResp["content"].(map[string]any); ok && content != nil {
									targetContent = content
								}
							}

							if targetContent != nil {
								existingParts, _ := targetContent["parts"].([]any)
								targetContent["parts"] = append(existingParts, dataParts...)
								targetContent["role"] = "model"
								ev0["content"] = targetContent
							} else {
								ev0["content"] = map[string]any{
									"role":  "model",
									"parts": dataParts,
								}
							}
						}
					}
				}
			}
			enc := json.NewEncoder(rw)
			enc.SetEscapeHTML(false)
			err = enc.Encode(m)
		} else {
			err = helper.EmitJSON(rw, respObj)
		}
		if err != nil {
			e := fmt.Errorf("helper.EmitJSON() failed: %w", err)
			log.Print(e.Error())
			e = helper.EmitJSONError(rw, e)
			if e != nil {
				e = fmt.Errorf("helper.EmitJSONError() failed: %w", e)
				log.Print(e.Error())
			}
			break
		}
	}

	// Guarantee that any pending A2UI visual cards are emitted before the stream closes
	if pending := a2ui.PopPendingA2UIMessages(runReq.SessionID); len(pending) > 0 {
		dataParts := make([]any, 0, len(pending))
		for _, msg := range pending {
			dataParts = append(dataParts, a2ui.BuildA2UIDataPart(msg))
		}
		a2aResp := map[string]any{
			"session_id": runReq.SessionID,
			"events": []any{
				map[string]any{
					"author": "data_recon_agent",
					"content": map[string]any{
						"role":  "model",
						"parts": dataParts,
					},
					"id":        fmt.Sprintf("a2ui-%d", time.Now().UnixNano()),
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				},
			},
		}
		enc := json.NewEncoder(rw)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(a2aResp)
	}

	return nil
}

// decodeStreamingAgentRunWithEventsRequest decodes input.request_json and returns
// the caller-requested session_id before backend ADK session normalization.
func decodeStreamingAgentRunWithEventsRequest(req *models.StreamingAgentRunWithEventsRequest) (*models.StreamingAgentRunWithEventsRunRequest, string, error) {
	var runReq models.StreamingAgentRunWithEventsRunRequest
	if err := json.Unmarshal([]byte(req.Input.RequestJSON), &runReq); err != nil {
		return nil, "", fmt.Errorf("json.Unmarshal(input.request_json) failed: %w", err)
	}

	// PATCH: describe the request shape before the typed decode discards it.
	//
	// StreamingAgentRunWithEventsRunRequest models user_id, session_id and
	// message only, and encoding/json drops unknown fields without a word. A
	// 15.9MB request that produced a 3,658-token prompt and no file parts is
	// exactly what that looks like, so log which keys arrived and how big each
	// is -- there is no other way to see where uploaded bytes actually sit.
	logRequestShape(req.Input.RequestJSON, &runReq)

	return &runReq, runReq.SessionID, nil
}

// ensureBackendSession normalizes Gemini Enterprise requests so
// the runner always receives a backend ADK session ID.
//
// On the first turn, the embedded session_id is usually a Gemini Enterprise /
// Discovery Engine session resource:
//
//	projects/{project}/locations/global/collections/default_collection/engines/{engine}/sessions/{session}
//
// VertexAISessionService cannot use that resource name as a caller-provided
// backend session ID. This method treats the incoming session_id as either a
// backend ADK session ID returned by a previous response, or an external
// first-turn resource that needs a newly-created backend session.
func (s *streamingAgentRunWithEventsHandler) ensureBackendSession(ctx context.Context, req *models.StreamingAgentRunWithEventsRunRequest, requestedSessionID string) error {
	if requestedSessionID == "" {
		return nil
	}
	if req.UserID == "" {
		return fmt.Errorf("user_id is required for backend session handling")
	}
	if s.config == nil || s.config.SessionService == nil {
		return fmt.Errorf("session service is required for backend session handling")
	}

	getResp, err := s.config.SessionService.Get(ctx, &session.GetRequest{
		AppName:   s.agentEngineID,
		UserID:    req.UserID,
		SessionID: requestedSessionID,
	})
	if err == nil && getResp.Session != nil {
		req.SessionID = getResp.Session.ID()
		return nil
	}

	createResp, err := s.config.SessionService.Create(ctx, &session.CreateRequest{
		AppName: s.agentEngineID,
		UserID:  req.UserID,
	})
	if err != nil {
		return fmt.Errorf("sessionService.Create() failed: %w", err)
	}

	req.SessionID = createResp.Session.ID()
	return nil
}

// Name implements MethodHandler.
func (s *streamingAgentRunWithEventsHandler) Name() string {
	return s.methodName
}

var _ MethodHandler = (*streamingAgentRunWithEventsHandler)(nil)

// Metadata implements MethodHandler.
func (s *streamingAgentRunWithEventsHandler) Metadata() (*structpb.Struct, error) {
	classAsyncMethod, err := structpb.NewStruct(map[string]any{
		"api_mode": s.apiMode,
		"name":     s.methodName,
		"parameters": map[string]any{
			"properties": map[string]any{
				"request_json": map[string]any{
					"type": "string",
				},
			},
			"required": []any{
				"request_json",
			},
			"type": "object",
		},
		"description": `Streams responses asynchronously from the ADK application.

This method is primarily meant for invocation from Gemini Enterprise.

Args:
    request_json (str):
        Required. A JSON-encoded request object with:
        - user_id (str): Required. The user ID to run the agent for.
        - session_id (str): Optional. The session ID. Gemini Enterprise may
          send its Discovery Engine session resource on the first turn; later
          turns should use the backend ADK session ID returned in the response.
        - message (Content): Required. The user message, using the genai
          Content JSON shape, for example:
          {"role":"user","parts":[{"text":"Hi"}]}

`,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create %s: %w", s.Name(), err)
	}
	return classAsyncMethod, nil
}

func (s *streamingAgentRunWithEventsHandler) run(ctx context.Context, req *models.StreamingAgentRunWithEventsRunRequest, message *genai.Content, config *launcher.Config) (iter.Seq2[*session.Event, error], error) {
	rootAgent := config.AgentLoader.RootAgent()

	r, err := runner.New(runner.Config{
		AppName:           s.agentEngineID,
		Agent:             rootAgent,
		SessionService:    config.SessionService,
		ArtifactService:   config.ArtifactService,
		MemoryService:     config.MemoryService,
		PluginConfig:      config.PluginConfig,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %v", err)
	}

	// The path mirrors Python AdkApp.streaming_agent_run_with_events,
	// which does not force SSE mode. Sending both partial and final events here
	// causes Gemini Enterprise to render duplicate answer text.
	return r.Run(ctx, req.UserID, req.SessionID, message, agent.RunConfig{
		StreamingMode: agent.StreamingModeNone,
	}), nil
}

// logRequestShape reports the top-level keys of request_json with their sizes,
// and what the typed decode actually produced. Keys only -- the values run to
// megabytes.
func logRequestShape(requestJSON string, decoded *models.StreamingAgentRunWithEventsRunRequest) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(requestJSON), &raw); err != nil {
		log.Printf("[REQSHAPE] request_json is not an object: %v", err)
		return
	}
	// modelled: fields the request struct carries. recovered: fields the struct
	// does not model but we read from the raw JSON ourselves. Anything in
	// neither is genuinely discarded, which is what the tag needs to make
	// obvious -- "artifacts" sat in that third category and cost 15.9MB a turn.
	known := map[string]bool{"user_id": true, "session_id": true, "message": true}
	recovered := map[string]bool{"artifacts": true}
	var sb strings.Builder
	for k, v := range raw {
		tag := ""
		switch {
		case known[k]:
		case recovered[k]:
			tag = " (recovered by artifactsToParts)"
		default:
			tag = " UNMODELLED-DROPPED"
		}

		fmt.Fprintf(&sb, " %s=%dB%s", k, len(v), tag)
	}
	log.Printf("[REQSHAPE] request_json %d bytes, keys:%s", len(requestJSON), sb.String())

	parts := 0
	var kinds []string
	if decoded.Message.Parts != nil {
		parts = len(decoded.Message.Parts)
		for _, p := range decoded.Message.Parts {
			switch {
			case p == nil:
			case p.InlineData != nil:
				kinds = append(kinds, fmt.Sprintf("inline(%s,%dB)", p.InlineData.MIMEType, len(p.InlineData.Data)))
			case p.FileData != nil:
				kinds = append(kinds, fmt.Sprintf("fileref(%s,%s)", p.FileData.MIMEType, p.FileData.FileURI))
			case p.Text != "":
				kinds = append(kinds, fmt.Sprintf("text(%dB)", len(p.Text)))
			default:
				kinds = append(kinds, "other")
			}
		}
	}
	log.Printf("[REQSHAPE] decoded message: role=%q parts=%d %s",
		decoded.Message.Role, parts, strings.Join(kinds, " "))
}

// artifactsToParts folds Gemini Enterprise's uploaded files into the message.
//
// PATCH. GE sends uploads in a top-level "artifacts" key rather than in
// message.parts, and upstream models artifacts on the response only -- so on
// the request they were an unknown field, silently discarded by encoding/json.
// A 15.9MB request reached the agent as an 80-byte text message, and the agent
// correctly reported that no files had arrived.
//
// Turning them into inline parts on the message means everything downstream --
// filename recovery, content validation, session staging, [UPLOAD] logging --
// works unchanged, instead of needing a second ingest path.
//
// The wire shape is undocumented, so several spellings are accepted and the
// keys of anything unrecognised are logged. Keys only; the values are megabytes.
func artifactsToParts(artifacts []json.RawMessage, message *genai.Content, sessionID string) {
	if len(artifacts) == 0 {
		return
	}
	added := 0
	inlineTotal := 0
	usedThisTurn := map[string]bool{}
	for i, raw := range artifacts {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			log.Printf("[ARTIFACT] %d: not an object (%d bytes): %v", i, len(raw), err)
			continue
		}

		name, mime, b64 := extractArtifact(obj)
		if b64 == "" {
			log.Printf("[ARTIFACT] %d: no recognised data field (%d bytes). name=%q mime=%q shape=%s",
				i, len(raw), name, mime, describeShape(obj, 0))
			continue
		}

		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			data, err = base64.URLEncoding.DecodeString(b64)
		}
		if err != nil {
			log.Printf("[ARTIFACT] %d (%s): data is not base64: %v", i, name, err)
			continue
		}

		// Spill anything large to disk and carry a reference instead of the
		// bytes. The runner persists the user turn to the Vertex session
		// service before the agent runs, and that AppendEvent is capped at
		// 10MiB -- an 11.9MB deck failed the whole turn with
		// "Request payload size exceeds the limit: 10486784 bytes", three
		// times over, after decoding perfectly.
		//
		// A file:// part costs a few hundred bytes and lands in the callback's
		// existing FileData branch, which reads it straight back off disk. The
		// model never sees either form: the before-model callback replaces
		// both with a text placeholder.
		if len(data) > maxInlineArtifactBytes || inlineTotal+len(data) > maxInlineArtifactTotal {
			if uri, err := spillArtifact(sessionID, i, name, data, usedThisTurn); err == nil {
				log.Printf("[ARTIFACT] %d: name=%q mime=%q decoded=%d bytes -> %s "+
					"(too large to persist inline)", i, name, mime, len(data), uri)
				message.Parts = append(message.Parts, &genai.Part{
					FileData: &genai.FileData{FileURI: uri, MIMEType: mime, DisplayName: name},
				})
				added++
				continue
			} else {
				// Inline anyway: an oversized turn that fails loudly beats
				// dropping the user's file and reporting success.
				log.Printf("[ARTIFACT] %d (%s): cannot spill to disk (%v); falling back to "+
					"inline, which may exceed the session service limit", i, name, err)
			}
		}

		log.Printf("[ARTIFACT] %d: name=%q mime=%q decoded=%d bytes -> message part",
			i, name, mime, len(data))
		message.Parts = append(message.Parts, &genai.Part{
			InlineData: &genai.Blob{Data: data, MIMEType: mime, DisplayName: name},
		})
		inlineTotal += len(data)
		added++
	}
	if added > 0 {
		log.Printf("[ARTIFACT] folded %d of %d uploads into message.parts", added, len(artifacts))
	}
}

// The Vertex session service rejects an AppendEvent over 10MiB, and the runner
// persists the user turn before the agent ever runs. These bounds keep the
// persisted event comfortably under that ceiling with room for the JSON/base64
// inflation the wire encoding adds, both per file and across a multi-file turn.
const (
	maxInlineArtifactBytes = 1 << 20 // 1MiB per artifact
	maxInlineArtifactTotal = 4 << 20 // 4MiB across the turn
)

// stagingBaseDir is the parent of every session directory. A package var only
// so tests can redirect it away from the real /tmp.
var stagingBaseDir = "/tmp"

// SessionStagingDir returns the directory an upload for this session is staged
// in, creating it if needed.
//
// This is the agent's own session root, NOT a separate staging tree. One
// container serves every user, so a shared inbound directory would put one
// customer's documents where another session could reach them if any path
// handling downstream ever slipped. Staging into the session root instead means
// uploads inherit the isolation the tools already enforce -- sessionPath()
// reduces every model-supplied path to a base name inside this directory, so a
// session cannot name a path outside its own root however it is asked to.
//
// Exported so agentapp can assert it agrees with sessionRoot(); the two compute
// the same path and must not drift.
func SessionStagingDir(sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		// Never a fixed fallback name. One container serves every user, so a
		// shared "default_session" directory would collect the uploads of every
		// turn that arrived without a session id -- the exact cross-tenant
		// exposure that staging into the session root exists to prevent. An
		// unguessable directory is the safe answer; the cost is only that it is
		// not the session root, so the agent copies the file in rather than
		// finding it already there.
		var buf [16]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf("no session id and no randomness to substitute: %w", err)
		}
		sessionID = "anon-" + hex.EncodeToString(buf[:])
	}
	dir := filepath.Join(stagingBaseDir, filepath.Base(filepath.Clean(sessionID)))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create session dir: %w", err)
	}
	return dir, nil
}

// spillArtifact writes one upload into the session's own directory and returns
// a file:// URI for it.
func spillArtifact(sessionID string, index int, name string, data []byte, usedThisTurn map[string]bool) (string, error) {
	dir, err := SessionStagingDir(sessionID)
	if err != nil {
		return "", err
	}

	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "upload"
	}
	// Land on the plain name the tools look for. The index prefix is a last
	// resort, used only when that name is genuinely taken by a different file:
	// either another upload already claimed it this turn, or a file of a
	// different size is sitting there.
	//
	// Re-sending the same file on a later turn must NOT produce a second copy.
	// The agent already writes several name variants of each upload, and the
	// model wasted a whole reasoning step on "Agent_ AskHR.xlsx and
	// Agent_AskHR.xlsx ... a minor anomaly" -- more copies make that worse.
	path := filepath.Join(dir, base)
	if usedThisTurn[path] {
		path = filepath.Join(dir, fmt.Sprintf("%d-%s", index, base))
	} else if existing, err := os.Stat(path); err == nil && existing.Size() != int64(len(data)) {
		path = filepath.Join(dir, fmt.Sprintf("%d-%s", index, base))
	}
	usedThisTurn[path] = true
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", path, err)
	}
	return "file://" + path, nil
}

// metadataKeys are the fields that describe an artifact rather than carry it.
// Their values are excluded from the payload search: a short identifier like
// "deadbeef" is legal base64 and would otherwise be a candidate.
var metadataKeys = map[string]bool{
	"file_name": true, "fileName": true, "filename": true, "name": true,
	"display_name": true, "displayName": true, "mime_type": true, "mimeType": true,
	"content_type": true, "contentType": true, "version": true, "id": true,
	"etag": true, "checksum": true, "create_time": true, "createTime": true,
	"update_time": true, "updateTime": true,
}

// extractArtifact pulls a name, mime type and base64 body out of one artifact.
//
// This searches by STRUCTURE rather than by key name, because guessing names
// failed three times running against an undocumented wire format: first the
// artifact turned out to be {file_name, versions[]}, then the bytes turned out
// to be nested deeper again inside a version. Rather than chase another key,
// walk the whole object and take the largest decodable base64 string that is
// not sitting under a metadata key.
//
// That is sound for a file artifact: the payload is by construction far larger
// than any id or timestamp around it, so "biggest base64 blob" is the file no
// matter how the envelope is arranged. Strict decoding does most of the work --
// most metadata is not valid base64 at all -- and metadataKeys covers the rest.
//
// Names and mime types are collected from any depth with shallower matches
// preferred: the artifact's own file_name is the user's filename, while one
// nested inside a version is likelier an internal id.
func extractArtifact(obj map[string]any) (name, mime, b64 string) {
	bestDepth := 1 << 30
	var walk func(v any, key string, depth int)
	walk = func(v any, key string, depth int) {
		switch t := v.(type) {
		case map[string]any:
			for _, k := range []string{"file_name", "fileName", "filename", "name", "display_name", "displayName"} {
				if sv, ok := t[k].(string); ok && sv != "" && depth < bestDepth {
					name, bestDepth = sv, depth
				}
			}
			if mime == "" {
				for _, k := range []string{"mime_type", "mimeType", "content_type", "contentType"} {
					if sv, ok := t[k].(string); ok && sv != "" {
						mime = sv
					}
				}
			}
			for k, child := range t {
				walk(child, k, depth+1)
			}
		case []any:
			for _, child := range t {
				walk(child, key, depth+1)
			}
		case string:
			// The payload dwarfs every other string in the envelope.
			if len(t) > len(b64) && !metadataKeys[key] && decodesAsBase64(t) {
				b64 = t
			}
		}
	}
	walk(obj, "", 0)
	return name, mime, b64
}

// decodesAsBase64 reports whether s is valid standard base64. Strict decoding
// is deliberate: it rejects most metadata outright, since arbitrary text is
// rarely both base64-legal and correctly padded. Multi-megabyte payloads are
// screened on a prefix first so the full decode only runs on real candidates.
func decodesAsBase64(s string) bool {
	if len(s) < 4 || len(s)%4 != 0 {
		return false
	}
	limit := len(s)
	if limit > 256 {
		limit = 256
	}
	for i := 0; i < limit; i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '+', c == '/', c == '=':
		default:
			return false
		}
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// describeShape renders the key structure of an object, two levels deep and
// with a sample of any list, so an unrecognised layout can be read straight out
// of the log instead of needing another round trip. Keys only -- a value here
// can be fifteen megabytes.
func describeShape(v any, depth int) string {
	if depth > 4 {
		return "..."
	}
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+":"+describeShape(t[k], depth+1))
		}
		return "{" + strings.Join(parts, " ") + "}"
	case []any:
		if len(t) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%d x %s]", len(t), describeShape(t[0], depth+1))
	case string:
		return fmt.Sprintf("str(%d)", len(t))
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", t)
	}
}

// firstString returns the value of the first key present whose value is a
// non-empty string.
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
