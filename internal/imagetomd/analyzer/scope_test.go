package analyzer

import (
	"strings"
	"testing"
)

const phase1Round1AnswerFixture = `| 区分 | 取得値 | 根拠位置 |
|---|---|---|
| 列数 | 9列 | ヘッダセル数 |
| 行数 | 可視11行（ヘッダ1 + データ2 + 空欄8） | 横罫線の区切り数 |
| 全列名 | ` + "`No.` / `変更箇所` / `変更内容(変更理由)` / `Ver` / `作成・変更者` / `作成・変更日` / `承認者` / `承認日` / `備考`" + ` | ヘッダ行 |
| 取得済みデータ行 | ` + "`No.43`, `No.44`" + ` | 1列目 No. 欄 |
| 入れ子/詳細リスト検出 | ` + "`No.43`" + `「変更内容(変更理由)」セル内 [SUB_TABLE_P01] | row |
| 入れ子/詳細リスト検出 | ` + "`No.44`" + `「変更内容(変更理由)」セル内 [SUB_TABLE_P02] | row |`

func TestExtractVisibleScopeFromPhase1Answer_No43And44(t *testing.T) {
	t.Parallel()
	log := PhaseLog{
		PhaseNum: 1,
		Rounds: []RoundLog{
			{Answer: phase1Round1AnswerFixture},
		},
	}
	scope := ExtractVisibleScopeFromPhase1(log)
	got := strings.Join(scope.VisibleRowIDs, ",")
	if got != "43,44" {
		t.Fatalf("got row IDs %q want 43,44", got)
	}
}

func TestExtractVisibleScopeFromPhase1Answer_VisibleRowCount(t *testing.T) {
	t.Parallel()
	log := PhaseLog{
		Rounds: []RoundLog{{Answer: phase1Round1AnswerFixture}},
	}
	scope := ExtractVisibleScopeFromPhase1(log)
	if scope.ColumnCount != 9 {
		t.Fatalf("column count got %d want 9", scope.ColumnCount)
	}
	if scope.VisibleBlankRows != 8 {
		t.Fatalf("blank rows got %d want 8", scope.VisibleBlankRows)
	}
}

func TestExtractVisibleScopeFromPhase1Answer_NestedPlaceholders(t *testing.T) {
	t.Parallel()
	log := PhaseLog{
		Rounds: []RoundLog{{Answer: phase1Round1AnswerFixture}},
	}
	scope := ExtractVisibleScopeFromPhase1(log)
	if len(scope.NestedPlaceholders) != 2 {
		t.Fatalf("placeholders got %#v", scope.NestedPlaceholders)
	}
}

func TestExtractVisibleScope_EmptyPhaseLogReturnsEmptyScope(t *testing.T) {
	t.Parallel()
	scope := ExtractVisibleScopeFromPhase1(PhaseLog{})
	if !scope.IsEmpty() {
		t.Fatalf("expected empty scope, got %#v", scope)
	}
}

func TestBuildPromptKnownInfo_Round1ScopeOnly(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{VisibleRowIDs: []string{"43", "44"}, ColumnCount: 9}
	got := buildPromptKnownInfo(scope, "long answer text", 1)
	if !strings.Contains(got, "[画像可視スコープ]") || !strings.Contains(got, "43, 44") {
		t.Fatalf("round1 should be scope summary only: %s", got)
	}
	if strings.Contains(got, "long answer") {
		t.Fatalf("round1 must not include previous answer: %s", got)
	}
}

func TestBuildPromptKnownInfo_Round2IncludesLatestDeltaOnly(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{VisibleRowIDs: []string{"43", "44"}}
	prev := strings.Repeat("x", 3000)
	got := buildPromptKnownInfo(scope, prev, 2)
	if !strings.Contains(got, "[直近ラウンド回答]") {
		t.Fatalf("round2 missing latest answer block: %s", got)
	}
	if len(got) > len(scope.SummaryText())+maxLatestAnswerInPrompt+100 {
		t.Fatalf("known info too long: %d chars", len(got))
	}
}

func TestBuildPromptKnownInfo_NoQuadraticGrowth(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{VisibleRowIDs: []string{"43", "44"}}
	r1 := strings.Repeat("a", 5000)
	r4 := strings.Repeat("b", 5000)
	known5 := buildPromptKnownInfo(scope, r4, 5)
	limit := len(scope.SummaryText()) + maxLatestAnswerInPrompt + 64
	if len(known5) > limit {
		t.Fatalf("round5 known %d exceeds limit %d", len(known5), limit)
	}
	_ = r1
}

func TestScopeSummaryTextContainsRowIDs(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{
		VisibleRowIDs:      []string{"43", "44"},
		ColumnCount:        9,
		VisibleBlankRows:   8,
		NestedPlaceholders: []string{"[SUB_TABLE_P01]"},
	}
	got := scope.SummaryText()
	for _, want := range []string{"43, 44", "列数: 9", "可視空欄行: 8", "SUB_TABLE_P01", "出力禁止"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in summary: %s", want, got)
		}
	}
}
