package dialog

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestJSONTransportAskAndReceive(t *testing.T) {
	in := strings.NewReader(`{"role":"user","type":"answer","text":"ok","values":{"name":"A"}}` + "\n")
	var out bytes.Buffer
	tr := NewJSONTransport(in, &out)
	if err := tr.Send(context.Background(), Message{Role: RoleAssistant, Type: TypeQuestion, Prompt: "q"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"type":"question"`) {
		t.Fatalf("out=%s", out.String())
	}
	msg, err := tr.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Values["name"] != "A" {
		t.Fatalf("%+v", msg)
	}
}

func TestTextTransportAskAndReceive(t *testing.T) {
	in := strings.NewReader("name=Bob\n")
	var errOut bytes.Buffer
	tr := NewTextTransport(in, &errOut)
	if err := tr.Send(context.Background(), Message{Role: RoleAssistant, Type: TypeQuestion, Prompt: "Need fields", Fields: []FieldSpec{{ID: "name", Label: "Name"}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "[question]") {
		t.Fatalf("%s", errOut.String())
	}
	msg, err := tr.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != TypeAnswer || msg.Values["name"] != "Bob" {
		t.Fatalf("%+v", msg)
	}
}
