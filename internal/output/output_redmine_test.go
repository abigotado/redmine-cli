package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/abigotado/redmine-cli/internal/errx"
)

type testRow struct {
	ID   int
	Name string
	Long string
}

func (row testRow) Fields() []Field {
	return []Field{
		{Name: "id", Value: "1", Raw: row.ID},
		{Name: "name", Value: row.Name, Raw: row.Name},
		{Name: "long", Value: row.Long, Raw: row.Long, OnRequest: true},
	}
}

func TestV1SchemaIsPinned(t *testing.T) {
	t.Parallel()
	assertJSONTags(t, reflect.TypeOf(Envelope{}), []string{"ok", "v", "data", "meta", "error", "hint"})
	assertJSONTags(t, reflect.TypeOf(Meta{}), []string{"count", "truncated", "total_count", "offset", "limit", "next_cursor", "profile", "base_url"})
	assertJSONTags(t, reflect.TypeOf(ErrorBody{}), []string{"code", "message", "candidates", "did_you_mean", "retry_after"})

	metaType := reflect.TypeOf(Meta{})
	for _, name := range []string{"Count", "Truncated", "TotalCount", "Offset", "Limit"} {
		field, ok := metaType.FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Pointer {
			t.Fatalf("Meta.%s must remain a pointer to preserve zero/false presence", name)
		}
	}
	errorType := reflect.TypeOf(ErrorBody{})
	retry, _ := errorType.FieldByName("RetryAfter")
	if retry.Type.Kind() != reflect.String {
		t.Fatalf("ErrorBody.RetryAfter type = %s, want string", retry.Type)
	}
}

func assertJSONTags(t *testing.T, typ reflect.Type, want []string) {
	t.Helper()
	got := make([]string, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		tag := strings.Split(typ.Field(index).Tag.Get("json"), ",")[0]
		got = append(got, tag)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s JSON tags = %v, want %v", typ.Name(), got, want)
	}
}

func TestSuccessPageEmitsOneCompleteEnvelope(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	writer := &Writer{Format: FormatJSON, Out: &stdout, Err: &bytes.Buffer{}}
	writer.WithContext("work", "https://redmine.example")
	err := writer.SuccessPage([]testRow{{ID: 1, Name: "One", Long: "hidden"}}, Page{
		TotalCount: 2, Offset: 0, Limit: 1, Truncated: true, NextCursor: "opaque",
	})
	if err != nil {
		t.Fatalf("SuccessPage() error = %v", err)
	}
	if strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("stdout is not exactly one JSON line: %q", stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	meta := envelope["meta"].(map[string]any)
	for key, want := range map[string]any{
		"count": float64(1), "truncated": true, "total_count": float64(2),
		"offset": float64(0), "limit": float64(1), "next_cursor": "opaque",
		"profile": "work", "base_url": "https://redmine.example",
	} {
		if meta[key] != want {
			t.Fatalf("meta[%s] = %#v, want %#v", key, meta[key], want)
		}
	}
	data := envelope["data"].([]any)[0].(map[string]any)
	if _, ok := data["long"]; ok {
		t.Fatal("on-request field appeared in default projection")
	}
}

func TestProjectionAndRawRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		format  Format
		fields  []string
		wantErr bool
	}{
		{name: "known projection", format: FormatJSON, fields: []string{"id", "long"}},
		{name: "unknown field", format: FormatJSON, fields: []string{"token"}, wantErr: true},
		{name: "raw rejects fields", format: FormatRaw, fields: []string{"id"}, wantErr: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &Writer{Format: testCase.format, Fields: testCase.fields, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
			err := writer.Success(testRow{ID: 1, Name: "One", Long: "detail"})
			if (err != nil) != testCase.wantErr {
				t.Fatalf("Success() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestValidateRejectsUnknownFieldsBeforeRendering(t *testing.T) {
	t.Parallel()
	writer := &Writer{Format: FormatJSON, Fields: []string{"token"}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	if err := writer.Validate(testRow{}); err == nil {
		t.Fatal("Validate() accepted an unknown field")
	}
}

func TestTextOutputEscapesTerminalControls(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	writer := &Writer{Format: FormatText, Out: &stdout, Err: &stderr}
	if err := writer.Success(testRow{ID: 1, Name: "safe\x1b]52;clipboard\a\u202ereversed", Long: ""}); err != nil {
		t.Fatalf("Success() error = %v", err)
	}
	writer.Failure((&errx.Error{
		Code: errx.CodeAmbiguous, Reason: "AMBIGUOUS", Message: "bad\x1b[2Jvalue\u202e", Hint: "pick\u202e",
		Candidates: []errx.Candidate{{ID: "1\u202e", Name: "candidate\u202e"}},
	}))
	for name, value := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String()} {
		if strings.ContainsAny(value, "\x1b\a\u202e") {
			t.Fatalf("%s contains terminal controls: %q", name, value)
		}
		if !strings.Contains(value, "\\x1b") {
			t.Fatalf("%s did not visibly escape ESC: %q", name, value)
		}
		if !strings.Contains(value, "\\u202e") {
			t.Fatalf("%s did not visibly escape bidi control: %q", name, value)
		}
	}
}

func TestFailureUsesDurationStringAndNoMeta(t *testing.T) {
	t.Parallel()
	var stdout bytes.Buffer
	writer := &Writer{Format: FormatJSON, Out: &stdout, Err: &bytes.Buffer{}}
	code := writer.Failure(errx.Retryable("RATE_LIMITED", 2*time.Second, "slow down"))
	if code != errx.CodeRetryable {
		t.Fatalf("code = %d", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if _, ok := envelope["meta"]; ok {
		t.Fatal("failure envelope unexpectedly contains meta")
	}
	errorBody := envelope["error"].(map[string]any)
	if errorBody["retry_after"] != "2s" {
		t.Fatalf("retry_after = %#v", errorBody["retry_after"])
	}
}
