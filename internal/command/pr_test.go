package command

import (
	"strings"
	"testing"
)

func TestPullRequestCreateArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  PullRequestRequest
		want []string
	}{
		{
			name: "gh",
			req:  PullRequestRequest{CLI: "gh", Branch: "feature", Title: "Title", Body: "Body"},
			want: []string{"pr", "create", "--title", "Title", "--body", "Body", "--head", "feature"},
		},
		{
			name: "glab",
			req:  PullRequestRequest{CLI: "glab", Branch: "feature", Title: "Title", Body: "Body"},
			want: []string{"mr", "create", "--title", "Title", "--description", "Body", "--source-branch", "feature", "--yes"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PullRequestCreateArgs(tc.req)
			if err != nil {
				t.Fatalf("PullRequestCreateArgs() error = %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("PullRequestCreateArgs() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
