package git

type ChangeStatus string

const (
	Added     ChangeStatus = "added"
	Modified  ChangeStatus = "modified"
	Deleted   ChangeStatus = "deleted"
	Renamed   ChangeStatus = "renamed"
	Untracked ChangeStatus = "untracked"
)

type FileChange struct {
	Path      string
	OldPath   string
	Status    ChangeStatus
	Additions int
	Deletions int
	Binary    bool
}

type Worktree struct {
	Path          string
	Branch        string
	Head          string
	Current       bool
	Primary       bool
	DefaultBranch bool
	Protected     bool
}

type LineStat struct {
	Additions int
	Deletions int
	Binary    bool
}
