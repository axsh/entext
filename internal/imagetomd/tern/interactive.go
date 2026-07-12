package tern

import (
	"fmt"
	"strings"
	"unicode/utf8"

	arcticclient "github.com/axsh/arctic-tern/client/v1"
)

const UnattendedAutoResponse = "無人バッチ実行です。確認や追加質問は不要です。添付画像を忠実に Markdown 化し、質問せずテーブルまたはリストを即時出力してください。"

type AgentGuardEvent struct {
	Kind          string `json:"kind"`
	PromptID      string `json:"prompt_id,omitempty"`
	ContentPrefix string `json:"content_prefix,omitempty"`
	ChoicesCount  int    `json:"choices_count,omitempty"`
	PickedIndex   int    `json:"picked_index,omitempty"`
	AutoResponse  bool   `json:"auto_response"`
}

type UnattendedInputHandler struct {
	maxResponses int
	progress     ProgressFunc
	responseCount int
	events       []AgentGuardEvent
}

func NewUnattendedInputHandler(maxResponses int, progress ProgressFunc) *UnattendedInputHandler {
	if maxResponses <= 0 {
		maxResponses = 3
	}
	return &UnattendedInputHandler{
		maxResponses: maxResponses,
		progress:     progress,
	}
}

func (h *UnattendedInputHandler) Handle(ev arcticclient.UserInputRequiredEvent) (string, error) {
	if h.responseCount >= h.maxResponses {
		return "", ErrInteractiveInputRequired
	}
	h.responseCount++

	event := AgentGuardEvent{
		Kind:          "user_input_required",
		PromptID:      ev.PromptID,
		ContentPrefix: truncateRunes(ev.Content, 120),
		ChoicesCount:  len(ev.Choices),
		AutoResponse:  true,
	}

	var response string
	if len(ev.Choices) > 0 {
		response = ev.Choices[0]
		event.PickedIndex = 0
		if h.progress != nil {
			h.progress("step=agent_guard kind=user_input_required prompt_id=%s choices=%d picked=0 auto_response=true", ev.PromptID, len(ev.Choices))
		}
	} else {
		response = UnattendedAutoResponse
		event.PickedIndex = -1
		if h.progress != nil {
			h.progress("step=agent_guard kind=user_input_required prompt_id=%s choices=0 auto_response=true", ev.PromptID)
		}
	}

	h.events = append(h.events, event)
	return response, nil
}

func (h *UnattendedInputHandler) Events() []AgentGuardEvent {
	if len(h.events) == 0 {
		return nil
	}
	out := make([]AgentGuardEvent, len(h.events))
	copy(out, h.events)
	return out
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= maxRunes {
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}

func (h *UnattendedInputHandler) String() string {
	return fmt.Sprintf("UnattendedInputHandler(responses=%d/%d)", h.responseCount, h.maxResponses)
}
