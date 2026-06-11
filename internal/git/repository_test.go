package git

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.errs[key]; err != nil {
		return "", err
	}
	return f.outputs[key], nil
}

func TestRepositoryChangesCombinesStatusAndNumstat(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"git status --porcelain=v1 -z":  " M README.md\x00?? scratch.txt\x00",
		"git diff --numstat -z HEAD --": "4\t2\tREADME.md\x00",
	}}
	repo := Repository{Dir: ".", Runner: runner}

	got, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}

	want := []FileChange{
		{Path: "README.md", Status: Modified, Additions: 4, Deletions: 2},
		{Path: "scratch.txt", Status: Untracked},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes() = %#v, want %#v", got, want)
	}
}

func TestRepositoryDiffReturnsUntrackedMessage(t *testing.T) {
	repo := Repository{}

	got, err := repo.Diff(context.Background(), FileChange{Path: "scratch.txt", Status: Untracked})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !strings.Contains(got, "Untracked file: scratch.txt") {
		t.Fatalf("Diff() = %q, want untracked message", got)
	}
}

func TestRepositoryChangesWrapsStatusError(t *testing.T) {
	runner := &fakeRunner{
		errs: map[string]error{"git status --porcelain=v1 -z": errors.New("not a repo")},
	}
	repo := Repository{Runner: runner}

	_, err := repo.Changes(context.Background())
	if err == nil || !strings.Contains(err.Error(), "git status") {
		t.Fatalf("Changes() error = %v, want git status context", err)
	}
}
