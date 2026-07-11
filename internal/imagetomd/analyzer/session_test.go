package analyzer

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSessionLogSnapshotIncludesInProgressPhase(t *testing.T) {
	t.Parallel()
	base := &SessionLog{
		ImagePath: "a.png",
		Phases: []PhaseLog{
			{PhaseNum: 1, PhaseName: "done", ExitReason: "soft_limit"},
		},
	}
	inProgress := &PhaseLog{
		PhaseNum:  2,
		PhaseName: "current",
		Rounds: []RoundLog{
			{GapAssessment: "gap", Sufficient: false},
		},
	}
	snap := base.Snapshot(inProgress)
	if len(snap.Phases) != 2 {
		t.Fatalf("got %d phases want 2", len(snap.Phases))
	}
	if snap.Phases[1].PhaseNum != 2 {
		t.Fatalf("second phase should be in-progress phase")
	}
}

func TestSessionLogSaveWritesAtomically(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := &SessionLog{
		ImagePath: "a.png",
		StartedAt: time.Now().UTC(),
		Status:    "in_progress",
	}
	if err := log.Save(dir, "sample"); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	target := filepath.Join(dir, "sample_session.json")
	if _, err := os.Stat(target + ".tmp"); err == nil {
		t.Fatalf("tmp file should be renamed away")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(string(data), `"status": "in_progress"`) {
		t.Fatalf("expected status in json: %s", string(data))
	}
}
