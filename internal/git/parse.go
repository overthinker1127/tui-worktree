package git

import (
	"fmt"
	"strconv"
	"strings"
)

func ParsePorcelainStatus(output string) ([]FileChange, error) {
	if strings.Contains(output, "\x00") {
		return parsePorcelainStatusZ(output)
	}

	var changes []FileChange
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 4 {
			return nil, fmt.Errorf("parse status line %q: too short", line)
		}

		code := line[:2]
		path := strings.TrimSpace(line[3:])
		change := FileChange{Path: path, Status: statusFromCode(code)}

		if change.Status == Renamed {
			oldPath, newPath, ok := strings.Cut(path, " -> ")
			if ok {
				change.OldPath = oldPath
				change.Path = newPath
			}
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func parsePorcelainStatusZ(output string) ([]FileChange, error) {
	records := splitNUL(output)
	changes := make([]FileChange, 0, len(records))
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			return nil, fmt.Errorf("parse status record %q: too short", record)
		}

		code := record[:2]
		change := FileChange{
			Path:   record[3:],
			Status: statusFromCode(code),
		}
		if change.Status == Renamed {
			if i+1 >= len(records) {
				return nil, fmt.Errorf("parse rename record %q: missing old path", record)
			}
			change.OldPath = records[i+1]
			i++
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func ParseNumstat(output string) (map[string]LineStat, error) {
	if strings.Contains(output, "\x00") {
		return parseNumstatZ(output)
	}

	stats := make(map[string]LineStat)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse numstat line %q: want 3 fields", line)
		}

		path := finalPath(parts[2])
		if parts[0] == "-" && parts[1] == "-" {
			stats[path] = LineStat{Binary: true}
			continue
		}

		additions, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parse additions for %q: %w", path, err)
		}
		deletions, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse deletions for %q: %w", path, err)
		}
		stats[path] = LineStat{Additions: additions, Deletions: deletions}
	}
	return stats, nil
}

func parseNumstatZ(output string) (map[string]LineStat, error) {
	stats := make(map[string]LineStat)
	records := splitNUL(output)
	for i := 0; i < len(records); i++ {
		record := records[i]
		stat, path, err := parseNumstatRecord(record)
		if err != nil {
			return nil, err
		}
		if path == "" {
			if i+2 >= len(records) {
				return nil, fmt.Errorf("parse rename numstat record %q: missing old/new paths", record)
			}
			path = records[i+2]
			i += 2
		}
		stats[path] = stat
	}
	return stats, nil
}

func parseNumstatRecord(record string) (LineStat, string, error) {
	parts := strings.SplitN(record, "\t", 3)
	if len(parts) != 3 {
		return LineStat{}, "", fmt.Errorf("parse numstat line %q: want 3 fields", record)
	}

	path := finalPath(parts[2])
	if parts[0] == "-" && parts[1] == "-" {
		return LineStat{Binary: true}, path, nil
	}

	additions, err := strconv.Atoi(parts[0])
	if err != nil {
		return LineStat{}, "", fmt.Errorf("parse additions for %q: %w", path, err)
	}
	deletions, err := strconv.Atoi(parts[1])
	if err != nil {
		return LineStat{}, "", fmt.Errorf("parse deletions for %q: %w", path, err)
	}
	return LineStat{Additions: additions, Deletions: deletions}, path, nil
}

func ApplyLineStats(changes []FileChange, stats map[string]LineStat) []FileChange {
	out := make([]FileChange, len(changes))
	copy(out, changes)
	for i := range out {
		stat, ok := stats[out[i].Path]
		if !ok {
			continue
		}
		out[i].Additions = stat.Additions
		out[i].Deletions = stat.Deletions
		out[i].Binary = stat.Binary
	}
	return out
}

func ParseWorktreeList(output string, currentPath string) ([]Worktree, error) {
	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	worktrees := make([]Worktree, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		worktree := Worktree{}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				worktree.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "HEAD "):
				worktree.Head = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				worktree.Branch = shortRef(strings.TrimPrefix(line, "branch "))
			case line == "detached":
				worktree.Branch = "detached"
			}
		}
		if worktree.Path == "" {
			return nil, fmt.Errorf("parse worktree block %q: missing path", block)
		}
		worktree.Current = worktree.Path == currentPath
		worktree.Primary = len(worktrees) == 0
		worktrees = append(worktrees, worktree)
	}
	return worktrees, nil
}

func splitNUL(output string) []string {
	raw := strings.Split(output, "\x00")
	records := make([]string, 0, len(raw))
	for _, record := range raw {
		if record == "" {
			continue
		}
		records = append(records, record)
	}
	return records
}

func statusFromCode(code string) ChangeStatus {
	if strings.Contains(code, "R") {
		return Renamed
	}
	if strings.Contains(code, "?") {
		return Untracked
	}
	if strings.Contains(code, "D") {
		return Deleted
	}
	if strings.Contains(code, "A") {
		return Added
	}
	return Modified
}

func finalPath(path string) string {
	_, after, ok := strings.Cut(path, " => ")
	if ok {
		return after
	}
	return path
}

func shortRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}
