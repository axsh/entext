package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SessionLog struct {
	ImagePath   string     `json:"image_path"`
	StartedAt   time.Time  `json:"started_at"`
	Category    string     `json:"category,omitempty"`
	ShortPath   bool       `json:"short_path,omitempty"`
	Phases      []PhaseLog `json:"phases"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
}

type PhaseLog struct {
	PhaseNum   int        `json:"phase_num"`
	PhaseName  string     `json:"phase_name"`
	Goal       string     `json:"goal"`
	Rounds     []RoundLog `json:"rounds"`
	ExitReason string     `json:"exit_reason"`
}

func (p PhaseLog) HasNonEmptyAnswer() bool {
	for _, r := range p.Rounds {
		if strings.TrimSpace(r.Answer) != "" {
			return true
		}
	}
	return false
}

type RoundLog struct {
	KnownInfo     string `json:"known_info"`
	GapAssessment string `json:"gap_assessment"`
	Sufficient    bool   `json:"sufficient"`
	Question      string `json:"question"`
	Answer        string `json:"answer"`
}

func (s *SessionLog) Save(outputDir string, basename string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(outputDir, basename+"_session.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
