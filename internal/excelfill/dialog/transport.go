package dialog

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type Transport interface {
	Send(ctx context.Context, msg Message) error
	Receive(ctx context.Context) (Message, error)
}

type JSONTransport struct {
	In  *bufio.Scanner
	Out io.Writer
}

func NewJSONTransport(in io.Reader, out io.Writer) *JSONTransport {
	return &JSONTransport{In: bufio.NewScanner(in), Out: out}
}

func (t *JSONTransport) Send(ctx context.Context, msg Message) error {
	_ = ctx
	b, err := Encode(msg)
	if err != nil {
		return err
	}
	_, err = t.Out.Write(b)
	return err
}

func (t *JSONTransport) Receive(ctx context.Context) (Message, error) {
	_ = ctx
	if !t.In.Scan() {
		if err := t.In.Err(); err != nil {
			return Message{}, err
		}
		return Message{}, io.EOF
	}
	return DecodeLine(t.In.Text())
}

type TextTransport struct {
	In  *bufio.Scanner
	Out io.Writer
}

func NewTextTransport(in io.Reader, errOut io.Writer) *TextTransport {
	return &TextTransport{In: bufio.NewScanner(in), Out: errOut}
}

func (t *TextTransport) Send(ctx context.Context, msg Message) error {
	_ = ctx
	var b strings.Builder
	switch msg.Type {
	case TypeQuestion:
		b.WriteString("[question] ")
		b.WriteString(msg.Prompt)
		b.WriteString("\n")
		for _, f := range msg.Fields {
			b.WriteString(fmt.Sprintf("  - %s (%s)\n", f.ID, f.Label))
		}
	case TypeStatus:
		b.WriteString("[status] ")
		b.WriteString(msg.Status)
		b.WriteString("\n")
	case TypeVisualIssue:
		b.WriteString("[visual_issue]\n")
		for _, is := range msg.Issues {
			b.WriteString(fmt.Sprintf("  - %s %s %s: %s (%s)\n", is.Kind, is.Sheet, is.CellHint, is.Description, is.Suggestion))
		}
	case TypeContinueConfirm:
		b.WriteString("[continue_confirm] Retries exhausted. Continue? (yes/no)\n")
		b.WriteString(msg.Prompt)
		b.WriteString("\n")
	case TypeDone:
		b.WriteString("[done] ")
		b.WriteString(msg.OutputPath)
		b.WriteString("\n")
	case TypeError:
		b.WriteString("[error] ")
		b.WriteString(msg.Error)
		b.WriteString("\n")
	default:
		raw, err := Encode(msg)
		if err != nil {
			return err
		}
		_, err = t.Out.Write(raw)
		return err
	}
	_, err := io.WriteString(t.Out, b.String())
	return err
}

func (t *TextTransport) Receive(ctx context.Context) (Message, error) {
	_ = ctx
	if !t.In.Scan() {
		if err := t.In.Err(); err != nil {
			return Message{}, err
		}
		return Message{}, io.EOF
	}
	line := strings.TrimSpace(t.In.Text())
	lower := strings.ToLower(line)
	switch {
	case lower == "yes" || lower == "y" || lower == "continue":
		c := true
		return Message{Role: RoleUser, Type: TypeContinueDecision, Continue: &c, Text: line}, nil
	case lower == "no" || lower == "n":
		c := false
		return Message{Role: RoleUser, Type: TypeContinueDecision, Continue: &c, Text: line}, nil
	default:
		// Treat free text as answer; key/value "id=value" pairs optional.
		values := map[string]string{}
		if strings.Contains(line, "=") {
			for _, part := range strings.Split(line, ",") {
				kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
				if len(kv) == 2 {
					values[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}
		return Message{Role: RoleUser, Type: TypeAnswer, Text: line, Values: values}, nil
	}
}
