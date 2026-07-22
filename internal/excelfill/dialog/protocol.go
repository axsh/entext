package dialog

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Role string

const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
	RoleSystem    Role = "system"
)

type Type string

const (
	TypeQuestion         Type = "question"
	TypeAnswer           Type = "answer"
	TypeStatus           Type = "status"
	TypeVisualIssue      Type = "visual_issue"
	TypeContinueConfirm  Type = "continue_confirm"
	TypeContinueDecision Type = "continue_decision"
	TypeDone             Type = "done"
	TypeError            Type = "error"
)

type Message struct {
	Role              Role              `json:"role"`
	Type              Type              `json:"type"`
	Prompt            string            `json:"prompt,omitempty"`
	Fields            []FieldSpec       `json:"fields,omitempty"`
	Text              string            `json:"text,omitempty"`
	Values            map[string]string `json:"values,omitempty"`
	Issues            []VisualIssue     `json:"issues,omitempty"`
	Status            string            `json:"status,omitempty"`
	Continue          *bool             `json:"continue,omitempty"`
	AdditionalRetries *int              `json:"additional_retries,omitempty"`
	OutputPath        string            `json:"output_path,omitempty"`
	RetriesUsed       int               `json:"retries_used,omitempty"`
	Error             string            `json:"error,omitempty"`
}

type FieldSpec struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

type VisualIssue struct {
	Kind        string `json:"kind"`
	Sheet       string `json:"sheet,omitempty"`
	CellHint    string `json:"cell_hint,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

func Encode(msg Message) ([]byte, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func DecodeLine(line string) (Message, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Message{}, fmt.Errorf("empty dialog line")
	}
	var msg Message
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return Message{}, err
	}
	switch msg.Type {
	case TypeQuestion, TypeAnswer, TypeStatus, TypeVisualIssue, TypeContinueConfirm, TypeContinueDecision, TypeDone, TypeError:
	default:
		return Message{}, fmt.Errorf("unknown dialog type %q", msg.Type)
	}
	return msg, nil
}
