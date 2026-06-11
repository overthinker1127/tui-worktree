package git

import (
	"fmt"
	"strconv"
	"strings"
)

func ParsePorcelainStatus(output string) ([]FileChange, error) {
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

func ParseNumstat(output string) (map[string]LineStat, error) {
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
