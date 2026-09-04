package providers

import "testing"

func TestMergeUniquePreservesBaseAndAppendsNew(t *testing.T) {
	base := []string{"discord.com", "discord.gg"}
	extra := []string{"discord.gg", "gateway.discord.gg", "discord.com"}
	got := mergeUnique(base, extra)
	want := []string{"discord.com", "discord.gg", "gateway.discord.gg"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestMergeUniqueHandlesEmptySlices(t *testing.T) {
	if got := mergeUnique(nil, []string{"a.com"}); len(got) != 1 || got[0] != "a.com" {
		t.Fatalf("got %v", got)
	}
	if got := mergeUnique([]string{"a.com"}, nil); len(got) != 1 || got[0] != "a.com" {
		t.Fatalf("got %v", got)
	}
}
