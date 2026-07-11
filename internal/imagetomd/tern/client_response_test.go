package tern

import "testing"

func TestFinalizeResponsePrefersTextChunks(t *testing.T) {
	t.Parallel()

	got := finalizeResponse([]string{"| h1 | h2 |", "|---|---|"}, "I will analyze the image.")
	if got != "| h1 | h2 |\n|---|---|" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestFinalizeResponseFallsBackToResult(t *testing.T) {
	t.Parallel()

	got := finalizeResponse(nil, "final result")
	if got != "final result" {
		t.Fatalf("unexpected response: %q", got)
	}
}
