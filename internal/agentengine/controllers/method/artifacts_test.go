package method

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// Gemini Enterprise sends uploads in a top-level "artifacts" key, which upstream
// models on the response only. Unknown request fields are dropped silently by
// encoding/json, so a 15.9MB turn reached the agent as an 80-byte text message:
//
//	[REQSHAPE] keys: artifacts=15957438B UNMODELLED-DROPPED ... message=161B
//	[REQSHAPE] decoded message: role="user" parts=1 text(80B)
//
// The wire shape is undocumented, so the decoder accepts several spellings.

func rawArtifacts(t *testing.T, objs ...any) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	for _, o := range objs {
		b, err := json.Marshal(o)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out = append(out, b)
	}
	return out
}

func TestArtifactsBecomeMessageParts(t *testing.T) {
	deck := []byte{'P', 'K', 0x03, 0x04, 'd', 'e', 'c', 'k'}
	b64 := base64.StdEncoding.EncodeToString(deck)

	shapes := []struct {
		name string
		obj  any
	}{
		{"flat snake_case", map[string]any{
			"name": "deck.pptx", "mime_type": "application/vnd.openxmlformats-officedocument.presentationml.presentation", "data": b64}},
		{"flat camelCase", map[string]any{
			"displayName": "deck.pptx", "mimeType": "application/pptx", "bytes": b64}},
		{"nested inline_data", map[string]any{
			"inline_data": map[string]any{"display_name": "deck.pptx", "mime_type": "application/pptx", "data": b64}}},
		{"nested camel inlineData", map[string]any{
			"inlineData": map[string]any{"displayName": "deck.pptx", "mimeType": "application/pptx", "data": b64}}},
		{"filename + content", map[string]any{
			"filename": "deck.pptx", "content_type": "application/pptx", "content": b64}},
	}

	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "here it is"}}}
			artifactsToParts(rawArtifacts(t, s.obj), msg, "u/s")

			if len(msg.Parts) != 2 {
				t.Fatalf("got %d parts, want the text plus one file", len(msg.Parts))
			}
			got := msg.Parts[1]
			if got.InlineData == nil {
				t.Fatal("the artifact did not become an inline part")
			}
			if string(got.InlineData.Data) != string(deck) {
				t.Errorf("bytes = %q, want %q", got.InlineData.Data, deck)
			}
			if got.InlineData.DisplayName != "deck.pptx" {
				t.Errorf("display name = %q, want deck.pptx -- without it the file "+
					"cannot be addressed by name", got.InlineData.DisplayName)
			}
		})
	}
}

// TestSeveralArtifactsKeepTheirIdentities -- a turn carries a deck and two
// spreadsheets, and each must stay distinct. Collapsing them onto one name is
// exactly the bug that made six uploads overwrite each other previously.
func TestSeveralArtifactsKeepTheirIdentities(t *testing.T) {
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "three files"}}}

	artifactsToParts(rawArtifacts(t,
		map[string]any{"name": "deck.pptx", "data": enc("PK-deck")},
		map[string]any{"name": "agents.xlsx", "data": enc("PK-agents")},
		map[string]any{"name": "bugs.xlsx", "data": enc("PK-bugs")},
	), msg, "u/s")

	if len(msg.Parts) != 4 {
		t.Fatalf("got %d parts, want text plus three files", len(msg.Parts))
	}
	want := map[string]string{"deck.pptx": "PK-deck", "agents.xlsx": "PK-agents", "bugs.xlsx": "PK-bugs"}
	for _, p := range msg.Parts[1:] {
		body, ok := want[p.InlineData.DisplayName]
		if !ok {
			t.Errorf("unexpected artifact %q", p.InlineData.DisplayName)
			continue
		}
		if string(p.InlineData.Data) != body {
			t.Errorf("%s carries %q, want %q", p.InlineData.DisplayName, p.InlineData.Data, body)
		}
		delete(want, p.InlineData.DisplayName)
	}
	if len(want) != 0 {
		t.Errorf("missing artifacts: %v", want)
	}
}

func TestUnrecognisedArtifactIsSkippedNotFatal(t *testing.T) {
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "hi"}}}
	artifactsToParts(rawArtifacts(t,
		map[string]any{"totally": "unexpected", "shape": 1},
		map[string]any{"name": "ok.pptx", "data": base64.StdEncoding.EncodeToString([]byte("PK"))},
	), msg, "u/s")

	// The good one still lands; the odd one is logged with its keys, not fatal.
	if len(msg.Parts) != 2 {
		t.Fatalf("got %d parts, want the text plus the one decodable artifact", len(msg.Parts))
	}
	if msg.Parts[1].InlineData.DisplayName != "ok.pptx" {
		t.Errorf("wrong artifact survived: %q", msg.Parts[1].InlineData.DisplayName)
	}
}

func TestNoArtifactsLeavesMessageAlone(t *testing.T) {
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "no files"}}}
	artifactsToParts(nil, msg, "u/s")
	if len(msg.Parts) != 1 {
		t.Errorf("message was modified when there were no artifacts: %d parts", len(msg.Parts))
	}
}

// TestGeminiEnterpriseVersionsShape pins the layout production actually sends:
//
//	[ARTIFACT] 2: no recognised data field (15862362 bytes).
//	            name="[Ext] Intel AskHR - Features Demo 2.pptx" keys=[file_name versions]
//
// The bytes live in a version entry, and the user's filename lives on the
// artifact rather than the version, so the outer name has to win.
func TestGeminiEnterpriseVersionsShape(t *testing.T) {
	deck := []byte{'P', 'K', 0x03, 0x04, 'd', 'e', 'c', 'k'}
	b64 := base64.StdEncoding.EncodeToString(deck)

	for _, tc := range []struct {
		name string
		obj  any
	}{
		{"version carries data directly", map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"version": 0, "mime_type": "application/pptx", "data": b64}},
		}},
		{"version wraps inline_data", map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"inline_data": map[string]any{"mime_type": "application/pptx", "data": b64}}},
		}},
		{"latest version wins", map[string]any{
			"file_name": "deck.pptx",
			"versions": []any{
				map[string]any{"version": 0, "data": base64.StdEncoding.EncodeToString([]byte("OLD"))},
				map[string]any{"version": 1, "data": b64},
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "here"}}}
			artifactsToParts(rawArtifacts(t, tc.obj), msg, "u/s")

			if len(msg.Parts) != 2 {
				t.Fatalf("got %d parts, want the text plus the deck", len(msg.Parts))
			}
			got := msg.Parts[1].InlineData
			if string(got.Data) != string(deck) {
				t.Errorf("bytes = %q, want %q", got.Data, deck)
			}
			if got.DisplayName != "deck.pptx" {
				t.Errorf("display name = %q, want the artifact's file_name", got.DisplayName)
			}
		})
	}
}

// TestUnknownShapeIsDescribedNotGuessed -- when the layout is still not
// recognised the log must carry enough structure to fix it without another
// deploy-and-look cycle.
func TestUnknownShapeIsDescribedNotGuessed(t *testing.T) {
	shape := describeShape(map[string]any{
		"file_name": "deck.pptx",
		"versions":  []any{map[string]any{"version": 0, "payload": "abc"}},
	}, 0)

	for _, want := range []string{"file_name", "versions", "payload"} {
		if !strings.Contains(shape, want) {
			t.Errorf("shape description omits %q: %s", want, shape)
		}
	}
	// Values must never be logged; they run to megabytes.
	if strings.Contains(shape, "abc") {
		t.Errorf("the description leaks values: %s", shape)
	}
}

// TestStructuralSearchFindsArbitrarilyNestedBytes.
//
// The wire format defeated three rounds of key-name guessing: first
// {file_name, versions[]}, then bytes nested deeper inside a version. The
// decoder now searches by structure -- largest base64 string wins -- so the
// depth and the key names stop mattering.
func TestStructuralSearchFindsArbitrarilyNestedBytes(t *testing.T) {
	deck := make([]byte, 400)
	for i := range deck {
		deck[i] = byte('A' + i%26)
	}
	copy(deck, []byte{'P', 'K', 0x03, 0x04})
	b64 := base64.StdEncoding.EncodeToString(deck)

	for _, tc := range []struct {
		name string
		obj  map[string]any
	}{
		{"versions[].data as a string", map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"version": 0, "data": b64}},
		}},
		{"versions[].data.inline_data.data", map[string]any{
			"file_name": "deck.pptx",
			"versions": []any{map[string]any{"version": 0,
				"data": map[string]any{"inline_data": map[string]any{"data": b64}}}},
		}},
		{"buried five levels deep under unknown keys", map[string]any{
			"file_name": "deck.pptx",
			"versions": []any{map[string]any{
				"payload": map[string]any{"wrapper": map[string]any{
					"whatever": map[string]any{"blobbo": b64}}}}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "here"}}}
			artifactsToParts(rawArtifacts(t, tc.obj), msg, "u/s")

			if len(msg.Parts) != 2 {
				t.Fatalf("got %d parts, want the text plus the deck", len(msg.Parts))
			}
			got := msg.Parts[1].InlineData
			if string(got.Data) != string(deck) {
				t.Errorf("recovered %d bytes, want %d", len(got.Data), len(deck))
			}
			if got.DisplayName != "deck.pptx" {
				t.Errorf("display name = %q, want the artifact's file_name", got.DisplayName)
			}
		})
	}
}

// TestShallowestNameWins -- the artifact's file_name is the user's filename; a
// nested one is likely an internal id and must not shadow it.
func TestShallowestNameWins(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(make([]byte, 200))
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "x"}}}
	artifactsToParts(rawArtifacts(t, map[string]any{
		"file_name": "Intel AskHR.pptx",
		"versions":  []any{map[string]any{"name": "v0-abc123", "data": b64}},
	}), msg, "u/s")

	if len(msg.Parts) != 2 {
		t.Fatalf("got %d parts", len(msg.Parts))
	}
	if got := msg.Parts[1].InlineData.DisplayName; got != "Intel AskHR.pptx" {
		t.Errorf("display name = %q, want the outer file_name", got)
	}
}

// TestShortIdentifiersAreNotMistakenForPayload -- ids and timestamps are also
// base64-legal; only the large blob should win.
func TestShortIdentifiersAreNotMistakenForPayload(t *testing.T) {
	deck := make([]byte, 300)
	copy(deck, []byte{'P', 'K', 0x03, 0x04})
	b64 := base64.StdEncoding.EncodeToString(deck)

	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "x"}}}
	artifactsToParts(rawArtifacts(t, map[string]any{
		"file_name": "deck.pptx",
		"etag":      "abc123",
		"versions":  []any{map[string]any{"id": "v0", "checksum": "deadbeef", "data": b64}},
	}), msg, "u/s")

	if len(msg.Parts) != 2 {
		t.Fatalf("got %d parts", len(msg.Parts))
	}
	if len(msg.Parts[1].InlineData.Data) != len(deck) {
		t.Errorf("picked a %d-byte value; the payload is %d bytes",
			len(msg.Parts[1].InlineData.Data), len(deck))
	}
}

// The Vertex session service caps AppendEvent at 10MiB and the runner persists
// the user turn BEFORE the agent runs, so an upload carried inline fails the
// whole turn no matter how well it decoded. A real 11.9MB deck did exactly
// that, three attempts in a row:
//
//	[ARTIFACT] 2: ... decoded=11896455 bytes -> message part
//	failed to append event to sessionService: ... Request payload size exceeds
//	the limit: 10486784 bytes
//
// Large uploads must therefore leave the event as a reference, not as bytes.
func TestLargeUploadIsSpilledToDiskNotCarriedInline(t *testing.T) {
	stagingBaseDir = t.TempDir()

	big := make([]byte, maxInlineArtifactBytes+1)
	copy(big, []byte{'P', 'K', 0x03, 0x04})
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "big deck"}}}

	artifactsToParts(rawArtifacts(t, map[string]any{
		"file_name": "big.pptx",
		"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(big)}},
	}), msg, "session-1")

	if len(msg.Parts) != 2 {
		t.Fatalf("got %d parts, want the text plus the upload", len(msg.Parts))
	}
	got := msg.Parts[1]
	if got.InlineData != nil {
		t.Fatalf("upload was carried inline (%d bytes); the session append would "+
			"exceed 10MiB and fail the turn", len(got.InlineData.Data))
	}
	if got.FileData == nil {
		t.Fatal("upload became neither inline data nor a file reference -- it was lost")
	}
	if got.FileData.DisplayName != "big.pptx" {
		t.Errorf("display name = %q, want big.pptx: the callback prefers DisplayName "+
			"over the URI basename, so losing it renames the user's file",
			got.FileData.DisplayName)
	}

	// The agent's FileData branch reads this path straight off disk.
	path, ok := strings.CutPrefix(got.FileData.FileURI, "file://")
	if !ok {
		t.Fatalf("URI %q is not file://; the callback only resolves file:// and gs://",
			got.FileData.FileURI)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spilled file is not readable: %v", err)
	}
	if len(onDisk) != len(big) {
		t.Errorf("spilled %d bytes, want %d", len(onDisk), len(big))
	}
}

// A turn of individually-small files can still breach the cap collectively.
func TestManySmallUploadsSpillOnceTheTurnGetsTooBig(t *testing.T) {
	stagingBaseDir = t.TempDir()

	const each = 900 << 10 // under the per-file bound, over the turn bound at 5x
	var objs []any
	for i := range 5 {
		body := make([]byte, each)
		body[0] = byte('a' + i)
		objs = append(objs, map[string]any{
			"file_name": "f.xlsx",
			"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(body)}},
		})
	}

	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "five files"}}}
	artifactsToParts(rawArtifacts(t, objs...), msg, "session-1")

	if len(msg.Parts) != 6 {
		t.Fatalf("got %d parts, want the text plus five uploads", len(msg.Parts))
	}
	inline := 0
	for _, p := range msg.Parts[1:] {
		if p.InlineData != nil {
			inline += len(p.InlineData.Data)
		} else if p.FileData == nil {
			t.Error("an upload became neither inline data nor a file reference")
		}
	}
	if inline > maxInlineArtifactTotal {
		t.Errorf("kept %d bytes inline across the turn, over the %d budget",
			inline, maxInlineArtifactTotal)
	}
}

// Small uploads must stay inline: that path is unchanged and already works, and
// a PDF or image under the bound is passed to the model to read directly.
func TestSmallUploadStaysInline(t *testing.T) {
	stagingBaseDir = t.TempDir()

	small := make([]byte, 2048)
	copy(small, []byte{'P', 'K', 0x03, 0x04})
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "small"}}}

	artifactsToParts(rawArtifacts(t, map[string]any{
		"file_name": "small.xlsx",
		"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(small)}},
	}), msg, "session-1")

	if len(msg.Parts) != 2 {
		t.Fatalf("got %d parts", len(msg.Parts))
	}
	if msg.Parts[1].InlineData == nil {
		t.Fatal("a small upload was spilled to disk; it should stay inline")
	}
}

// One container serves every user, so two sessions must never stage into the
// same directory. Uploads land in the session root precisely so they inherit
// the isolation the tools already enforce, rather than depending on a second,
// parallel scheme that has to be kept correct separately.
func TestSpillDirectoriesAreScopedPerSession(t *testing.T) {
	stagingBaseDir = t.TempDir()

	big := make([]byte, maxInlineArtifactBytes+1)
	uriFor := func(sessionID string) string {
		msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "x"}}}
		artifactsToParts(rawArtifacts(t, map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(big)}},
		}), msg, sessionID)
		if len(msg.Parts) != 2 || msg.Parts[1].FileData == nil {
			t.Fatalf("no spilled part for %q", sessionID)
		}
		return msg.Parts[1].FileData.FileURI
	}

	a, b := uriFor("session-a"), uriFor("session-b")
	if filepath.Dir(a) == filepath.Dir(b) {
		t.Errorf("both sessions staged into %s; one user could read the other's "+
			"uploads", filepath.Dir(a))
	}
	if a == b {
		t.Errorf("both sessions resolved to the same file %s -- one overwrote the other", a)
	}
}

// Same filename twice in one turn must not collapse to a single file.
func TestSameNameTwiceKeepsBothFiles(t *testing.T) {
	stagingBaseDir = t.TempDir()

	mk := func(fill byte) any {
		body := make([]byte, maxInlineArtifactBytes+1)
		for i := range body {
			body[i] = fill
		}
		return map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(body)}},
		}
	}
	msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "two"}}}
	artifactsToParts(rawArtifacts(t, mk('A'), mk('B')), msg, "session-1")

	if len(msg.Parts) != 3 {
		t.Fatalf("got %d parts, want the text plus both uploads", len(msg.Parts))
	}
	first, second := msg.Parts[1].FileData, msg.Parts[2].FileData
	if first.FileURI == second.FileURI {
		t.Fatalf("both uploads wrote to %s; one overwrote the other", first.FileURI)
	}
	// Both keep the user-facing name; only the on-disk path disambiguates.
	for _, fd := range []*genai.FileData{first, second} {
		if fd.DisplayName != "deck.pptx" {
			t.Errorf("display name = %q, want deck.pptx", fd.DisplayName)
		}
	}
}

// A turn can arrive with files and no session id. Staging those under one
// fixed name would pool every anonymous first turn in the container into a
// single directory -- the cross-tenant exposure that staging into the session
// root exists to prevent.
func TestMissingSessionIDDoesNotPoolUploadsInOneDirectory(t *testing.T) {
	stagingBaseDir = t.TempDir()

	big := make([]byte, maxInlineArtifactBytes+1)
	dirFor := func() string {
		msg := &genai.Content{Role: "user", Parts: []*genai.Part{{Text: "x"}}}
		artifactsToParts(rawArtifacts(t, map[string]any{
			"file_name": "deck.pptx",
			"versions":  []any{map[string]any{"data": base64.StdEncoding.EncodeToString(big)}},
		}), msg, "")
		if len(msg.Parts) != 2 || msg.Parts[1].FileData == nil {
			t.Fatal("upload was not staged")
		}
		return filepath.Dir(strings.TrimPrefix(msg.Parts[1].FileData.FileURI, "file://"))
	}

	a, b := dirFor(), dirFor()
	if a == b {
		t.Errorf("two session-less turns both staged into %s; one caller could read "+
			"the other's uploads", a)
	}
	for _, d := range []string{a, b} {
		if filepath.Base(d) == "default_session" {
			t.Errorf("staged into the shared fallback %s", d)
		}
	}
}
