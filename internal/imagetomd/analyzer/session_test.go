package analyzer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionLogSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := &SessionLog{
		ImagePath: "a.png",
		StartedAt: time.Now().UTC(),
		Phases: []PhaseLog{
			{
				PhaseNum:  1,
				PhaseName: "Overview",
				Goal:      "goal",
				Rounds: []RoundLog{
					{
						KnownInfo:     "known",
						GapAssessment: "gap",
						Sufficient:    false,
						Question:      "q1",
						Answer:        "a1",
					},
				},
				ExitReason: "soft_limit",
			},
		},
	}

	if err := log.Save(dir, "sample"); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	target := filepath.Join(dir, "sample_session.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("session file must not be empty")
	}
}
