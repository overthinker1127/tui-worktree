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

type LineStat struct {
	Additions int
	Deletions int
	Binary    bool
}
