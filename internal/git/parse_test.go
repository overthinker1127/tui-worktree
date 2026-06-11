package git

import (
	"reflect"
	"testing"
)

func TestParsePorcelainStatus(t *testing.T) {
	input := "" +
		" M README.md\n" +
		"A  cmd/app/main.go\n" +
		"D  old.txt\n" +
		"R  before.go -> after.go\n" +
		"?? scratch.txt\n"

	got, err := ParsePorcelainStatus(input)
	if err != nil {
		t.Fatalf("ParsePorcelainStatus() error = %v", err)
	}

	want := []FileChange{
		{Path: "README.md", Status: Modified},
		{Path: "cmd/app/main.go", Status: Added},
		{Path: "old.txt", Status: Deleted},
		{Path: "after.go", OldPath: "before.go", Status: Renamed},
		{Path: "scratch.txt", Status: Untracked},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePorcelainStatus() = %#v, want %#v", got, want)
	}
}

func TestParsePorcelainStatusZHandlesSpacesAndRenames(t *testing.T) {
	input := " M a b.txt\x00RM new name.txt\x00old name.txt\x00"

	got, err := ParsePorcelainStatus(input)
	if err != nil {
		t.Fatalf("ParsePorcelainStatus() error = %v", err)
	}

	want := []FileChange{
		{Path: "a b.txt", Status: Modified},
		{Path: "new name.txt", OldPath: "old name.txt", Status: Renamed},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePorcelainStatus() = %#v, want %#v", got, want)
	}
}

func TestParseNumstat(t *testing.T) {
	input := "" +
		"10\t2\tREADME.md\n" +
		"-\t-\tasset.png\n" +
		"3\t1\told.go => new.go\n"

	got, err := ParseNumstat(input)
	if err != nil {
		t.Fatalf("ParseNumstat() error = %v", err)
	}

	want := map[string]LineStat{
		"README.md": {Additions: 10, Deletions: 2},
		"asset.png": {Binary: true},
		"new.go":    {Additions: 3, Deletions: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNumstat() = %#v, want %#v", got, want)
	}
}

func TestParseNumstatZHandlesSpaces(t *testing.T) {
	input := "1\t0\ta b.txt\x00-\t-\timage file.png\x00"

	got, err := ParseNumstat(input)
	if err != nil {
		t.Fatalf("ParseNumstat() error = %v", err)
	}

	want := map[string]LineStat{
		"a b.txt":        {Additions: 1},
		"image file.png": {Binary: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNumstat() = %#v, want %#v", got, want)
	}
}

func TestParseNumstatZHandlesRenameRecord(t *testing.T) {
	input := "0\t0\t\x00old name.txt\x00new name.txt\x00"

	got, err := ParseNumstat(input)
	if err != nil {
		t.Fatalf("ParseNumstat() error = %v", err)
	}

	want := map[string]LineStat{
		"new name.txt": {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNumstat() = %#v, want %#v", got, want)
	}
}

func TestApplyLineStats(t *testing.T) {
	changes := []FileChange{
		{Path: "README.md", Status: Modified},
		{Path: "asset.png", Status: Added},
	}
	stats := map[string]LineStat{
		"README.md": {Additions: 4, Deletions: 2},
		"asset.png": {Binary: true},
	}

	got := ApplyLineStats(changes, stats)

	want := []FileChange{
		{Path: "README.md", Status: Modified, Additions: 4, Deletions: 2},
		{Path: "asset.png", Status: Added, Binary: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyLineStats() = %#v, want %#v", got, want)
	}
}

func TestParseWorktreeList(t *testing.T) {
	input := "" +
		"worktree /repo\n" +
		"HEAD abc123\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /repo/.worktrees/feature\n" +
		"HEAD def456\n" +
		"branch refs/heads/feature\n"

	got, err := ParseWorktreeList(input, "/repo")
	if err != nil {
		t.Fatalf("ParseWorktreeList() error = %v", err)
	}

	want := []Worktree{
		{Path: "/repo", Branch: "main", Head: "abc123", Current: true, Primary: true},
		{Path: "/repo/.worktrees/feature", Branch: "feature", Head: "def456"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseWorktreeList() = %#v, want %#v", got, want)
	}
}
