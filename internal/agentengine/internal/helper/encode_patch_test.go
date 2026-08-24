package helper

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/genai"
)

// The reason this package is forked.
//
// ConvertSnake's type switch handled every signed integer width and no unsigned
// one, so a []byte -- reflected as a slice of uint8 -- failed with
// "unsupported type: uint8". genai.Part.ThoughtSignature is a []byte and Gemini
// attaches one to every tool call, so in production the encoder logged
//
//	failed to convert ... .parts.[]*.thoughtSignature.[]  err: unsupported type: uint8
//
// and returned the raw Go struct instead. That reaches Gemini Enterprise with
// PascalCase keys it cannot parse, and the turn renders as nothing.

func TestThoughtSignatureConverts(t *testing.T) {
	sig := []byte{0x01, 0x02, 0xff, 0x00, 0x7f}
	content := &genai.Content{Role: "model", Parts: []*genai.Part{
		{Text: "Hello", ThoughtSignature: sig},
	}}

	got := ConvertSnake(content)

	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("conversion fell back to the raw struct (%T); Gemini Enterprise "+
			"would receive PascalCase keys and render nothing", got)
	}
	parts, ok := m["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("parts missing or malformed: %#v", m["parts"])
	}
	part := parts[0].(map[string]any)

	// Bytes belong in JSON as base64, which is also what the Python ADK emits.
	want := base64.StdEncoding.EncodeToString(sig)
	if got := part["thought_signature"]; got != want {
		t.Errorf("thought_signature = %#v, want %q", got, want)
	}
	if part["text"] != "Hello" {
		t.Errorf("the visible text was lost: %#v", part["text"])
	}
}

// TestUnsignedIntegersConvert -- the omission was not specific to []byte; no
// unsigned width was handled at all.
func TestUnsignedIntegersConvert(t *testing.T) {
	type widths struct {
		A uint
		B uint8
		C uint16
		D uint32
		E uint64
	}
	got := ConvertSnake(widths{A: 1, B: 2, C: 3, D: 4, E: 5})
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("unsigned fields still fail conversion: %T", got)
	}
	for k, want := range map[string]uint64{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5} {
		if m[k] != want {
			t.Errorf("%s = %#v, want %d", k, m[k], want)
		}
	}
}

// TestSignedIntegersStillConvert -- the patch must not disturb what worked.
func TestSignedIntegersStillConvert(t *testing.T) {
	type signed struct{ A int32 }
	m, ok := ConvertSnake(signed{A: -7}).(map[string]any)
	if !ok || m["a"] != int64(-7) {
		t.Errorf("signed conversion regressed: %#v", m)
	}
}

// TestNonByteSlicesAreStillArrays -- only []byte becomes a string; every other
// slice must keep its shape.
func TestNonByteSlicesAreStillArrays(t *testing.T) {
	type holder struct{ Names []string }
	m := ConvertSnake(holder{Names: []string{"a", "b"}}).(map[string]any)
	arr, ok := m["names"].([]any)
	if !ok || len(arr) != 2 || arr[0] != "a" {
		t.Errorf("[]string was mangled: %#v (%v)", m["names"], reflect.TypeOf(m["names"]))
	}
}

// TestA2UIDataPartConvertsToNativeA2APart verifies that A2UI inline_data parts
// are transformed into native A2A DataPart envelopes (kind: "data") so Discovery Engine
// does not treat them as unsupported file attachments.
func TestA2UIDataPartConvertsToNativeA2APart(t *testing.T) {
	rawPayload := `<a2a_datapart_json>{"createSurface":{"surfaceId":"test-surface","catalogId":"https://a2ui.org/specification/v0_9/standard_catalog_definition.json"}}</a2a_datapart_json>`
	content := &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{
				InlineData: &genai.Blob{
					MIMEType: "application/json+a2ui",
					Data:     []byte(rawPayload),
				},
			},
		},
	}

	got := ConvertSnake(content)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	parts, ok := m["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 part, got: %#v", m["parts"])
	}

	part, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected part map, got: %T", parts[0])
	}

	// Must NOT contain inline_data (which Discovery Engine treats as an unsupported file attachment)
	if _, hasInline := part["inline_data"]; hasInline {
		t.Fatalf("expected inline_data to be deleted, got: %#v", part["inline_data"])
	}

	// Must be kind: "data" with metadata and parsed JSON data object (for Angular A2UI renderer)
	if part["kind"] != "data" {
		t.Fatalf("expected kind='data', got: %v", part["kind"])
	}

	// Must contain pure JSON text without <a2a_datapart_json> tags
	if textStr, hasText := part["text"].(string); !hasText || strings.Contains(textStr, "<a2a_datapart_json>") || !strings.Contains(textStr, "test-surface") {
		t.Fatalf("expected pure JSON text without <a2a_datapart_json>, got: %#v", part["text"])
	}

	metadata, ok := part["metadata"].(map[string]any)
	if !ok || metadata["mimeType"] != "application/json+a2ui" {
		t.Fatalf("expected metadata.mimeType='application/json+a2ui', got: %#v", part["metadata"])
	}

	dataObj, ok := part["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got: %T", part["data"])
	}

	createSurface, ok := dataObj["createSurface"].(map[string]any)
	if !ok || createSurface["surfaceId"] != "test-surface" {
		t.Fatalf("expected createSurface.surfaceId='test-surface', got: %#v", dataObj)
	}
}

// TestA2UITextPartConvertsToNativeA2APart verifies that if a text part contains
// legacy <a2a_datapart_json> tags, ConvertSnake unwraps it into a native A2A DataPart (kind: "data").
func TestA2UITextPartConvertsToNativeA2APart(t *testing.T) {
	rawPayload := `<a2a_datapart_json>{"createSurface":{"surfaceId":"text-surface-1","catalogId":"https://a2ui.org/specification/v0_9/standard_catalog_definition.json"}}</a2a_datapart_json>`
	content := &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{
				Text: rawPayload,
			},
		},
	}

	got := ConvertSnake(content)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	parts, ok := m["parts"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 part, got: %#v", m["parts"])
	}

	part, ok := parts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected part map, got: %T", parts[0])
	}

	if textStr, hasText := part["text"].(string); !hasText || strings.Contains(textStr, "<a2a_datapart_json>") || !strings.Contains(textStr, "text-surface-1") {
		t.Fatalf("expected pure JSON text without <a2a_datapart_json>, got: %#v", part["text"])
	}

	if part["kind"] != "data" {
		t.Fatalf("expected kind='data', got: %v", part["kind"])
	}

	metadata, ok := part["metadata"].(map[string]any)
	if !ok || metadata["mimeType"] != "application/json+a2ui" {
		t.Fatalf("expected metadata.mimeType='application/json+a2ui', got: %#v", part["metadata"])
	}

	dataObj, ok := part["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got: %T", part["data"])
	}

	createSurface, ok := dataObj["createSurface"].(map[string]any)
	if !ok || createSurface["surfaceId"] != "text-surface-1" {
		t.Fatalf("expected createSurface.surfaceId='text-surface-1', got: %#v", dataObj)
	}
}
