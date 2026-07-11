package exceltopdf

import "testing"

func TestParseSheetIndices(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "empty means all sheets", raw: "", want: nil},
		{name: "single sheet", raw: "1", want: []int{1}},
		{name: "multiple sheets", raw: "1,3,5", want: []int{1, 3, 5}},
		{name: "spaces are allowed", raw: " 1, 3 ,5 ", want: []int{1, 3, 5}},
		{name: "invalid letters", raw: "a,b", wantErr: true},
		{name: "invalid zero", raw: "0,1", wantErr: true},
		{name: "invalid negative", raw: "-1,2", wantErr: true},
		{name: "invalid trailing comma", raw: "1,", wantErr: true},
		{name: "duplicate", raw: "1,1", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseSheetIndices(tt.raw)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("%s: got len %d, want %d", tt.name, len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("%s: got[%d]=%d, want %d", tt.name, i, got[i], tt.want[i])
			}
		}
	}
}
