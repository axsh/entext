package dialog

import (
	"strings"
	"testing"
)

func TestMarshalQuestionAndParseAnswer(t *testing.T) {
	q := Message{
		Role:   RoleAssistant,
		Type:   TypeQuestion,
		Prompt: "Need name",
		Fields: []FieldSpec{{ID: "name", Label: "Name", Required: true}},
	}
	b, err := Encode(q)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeLine(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != TypeQuestion || got.Prompt != "Need name" || len(got.Fields) != 1 {
		t.Fatalf("%+v", got)
	}

	ansLine := `{"role":"user","type":"answer","values":{"name":"Alice"}}`
	ans, err := DecodeLine(ansLine)
	if err != nil {
		t.Fatal(err)
	}
	if ans.Values["name"] != "Alice" {
		t.Fatalf("%+v", ans)
	}
}

func TestParseContinueDecision(t *testing.T) {
	c := true
	n := 2
	b, err := Encode(Message{Role: RoleUser, Type: TypeContinueDecision, Continue: &c, AdditionalRetries: &n})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeLine(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Continue == nil || !*got.Continue || got.AdditionalRetries == nil || *got.AdditionalRetries != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestRejectUnknownType(t *testing.T) {
	_, err := DecodeLine(`{"role":"user","type":"nope"}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVisualIssueMessageRoundTrip(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		Type: TypeVisualIssue,
		Issues: []VisualIssue{{
			Kind: "overflow", Sheet: "Sheet1", CellHint: "B1", Description: "text clipped", Suggestion: "shorten",
		}},
	}
	b, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeLine(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Issues) != 1 || got.Issues[0].Kind != "overflow" {
		t.Fatalf("%+v", got)
	}
}
