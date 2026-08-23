package agentengine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// forkedFromADKVersion is the upstream release this tree was copied from:
// google.golang.org/adk/v2@v2.0.0, server/agentengine (plus the two launcher
// files from cmd/launcher).
//
// Update it only after re-diffing against the new upstream tree and carrying
// the patch forward.
const forkedFromADKVersion = "v2.0.0"

// TestForkIsStillAgainstTheRecordedADKVersion catches silent drift.
//
// Nothing in the Go toolchain links this copy to upstream's, so an ADK upgrade
// will not touch it and will not warn. This is runtime request-handling code:
// staying on a stale copy is worse here than for the CLI fork in cmd/adkgo.
//
// Check whether the encoder defect is fixed upstream on every bump. It was
// present and byte-identical in both v2.0.0 and v2.1.0; when it is finally
// fixed, delete this fork rather than carrying it forever.
func TestForkIsStillAgainstTheRecordedADKVersion(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("cannot read go.mod: %v", err)
	}
	m := regexp.MustCompile(`google\.golang\.org/adk/v2\s+(v[0-9][^\s]*)`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("google.golang.org/adk/v2 is not in go.mod; has the dependency moved?")
	}
	if got := m[1]; got != forkedFromADKVersion {
		t.Errorf(`the ADK was upgraded to %s but go/internal/agentengine is still the fork of %s.

This tree is a copy of upstream's server/agentengine, patched in
internal/helper/encode.go so that a []byte (genai.Part.ThoughtSignature)
serialises as base64 instead of failing with "unsupported type: uint8".

On this bump:
  1. Check whether upstream fixed convertSnake. If so, DELETE this fork and go
     back to google.golang.org/adk/v2/cmd/launcher/agentengine.
  2. If not, re-diff the tree, carry the patches marked "PATCH" forward, and set
     forkedFromADKVersion to %s.`, got, forkedFromADKVersion, got)
	}
}

// TestEncoderPatchIsStillPresent guards the reason the fork exists. Losing it
// in a merge would leave Gemini Enterprise rendering blank turns while every
// log looks healthy -- the failure mode that cost a full debugging session.
func TestEncoderPatchIsStillPresent(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("internal", "helper", "encode.go"))
	if err != nil {
		t.Fatalf("cannot read the forked encoder: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "base64.StdEncoding.EncodeToString") {
		t.Error("the []byte patch is gone; thought signatures will fail to " +
			"serialise and Gemini Enterprise will render empty turns")
	}
	if !strings.Contains(text, "reflect.Uint8") {
		t.Error("the unsigned-integer case is gone; any uint field fails conversion")
	}
	if strings.Count(text, "PATCH") < 2 {
		t.Error("the PATCH markers are gone; a future re-diff will not know what " +
			"was changed or why")
	}
}

// TestPayloadTruncationIsExplicit.
//
// Upstream reads exactly max_payload_size through an io.LimitReader, so an
// oversized request is silently cut mid-JSON and fails as an opaque unmarshal
// error. A turn carrying an uploaded deck plus two spreadsheets goes past the
// 10MiB default easily; in production it surfaced to the user as only
// "Reasoning Engine Execution failed", with no [TOKENS] or [UPLOAD] line
// because the agent never ran.
func TestPayloadTruncationIsExplicit(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("controllers", "agent_engine.go"))
	if err != nil {
		t.Fatalf("cannot read the forked controller: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "c.maxPayloadSize+1") {
		t.Error("the reader no longer looks past the limit, so an oversized " +
			"request is silently truncated instead of reported")
	}
	if !strings.Contains(text, "StatusRequestEntityTooLarge") {
		t.Error("an oversized request no longer returns 413; the caller cannot " +
			"tell a size limit from a malformed body")
	}
	if !strings.Contains(text, "max_payload_size") {
		t.Error("the error does not name the flag to raise")
	}
}
