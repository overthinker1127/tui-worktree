package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

const (
	prInputWidth = 48
	prBodyHeight = 5
)

type pr struct {
	title      textinput.Model
	body       textarea.Model
	focus      prFocus
	submitting bool
	forgeCLI   string
}

func (p *pr) reset(titleStyles textinput.Styles, bodyStyles textarea.Styles) {
	p.title = textinput.New()
	p.title.Prompt = ""
	p.title.Placeholder = "PR title"
	p.title.SetWidth(prInputWidth)
	p.title.SetStyles(titleStyles)
	p.title.Focus()

	p.body = textarea.New()
	p.body.Prompt = ""
	p.body.Placeholder = "PR description"
	p.body.ShowLineNumbers = false
	p.body.SetWidth(prInputWidth)
	p.body.SetHeight(prBodyHeight)
	p.body.SetStyles(bodyStyles)
	p.body.Blur()

	p.focus = prTitle
}

func (p *pr) toggleFocus() {
	if p.focus == prTitle {
		p.title.Blur()
		p.body.Focus()
		p.focus = prBody
		return
	}
	p.body.Blur()
	p.title.Focus()
	p.focus = prTitle
}

func (p *pr) updateInput(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	if p.focus == prTitle {
		p.title, cmd = p.title.Update(msg)
		return cmd
	}
	p.body, cmd = p.body.Update(msg)
	return cmd
}

func (p pr) request(worktree gitview.Worktree) (PullRequestRequest, error) {
	title := strings.TrimSpace(p.title.Value())
	if title == "" {
		return PullRequestRequest{}, fmt.Errorf("PR title is required")
	}
	if worktree.Path == "" {
		return PullRequestRequest{}, fmt.Errorf("no worktree selected")
	}
	if worktree.Branch == "" || worktree.Branch == "detached" {
		return PullRequestRequest{}, fmt.Errorf("selected worktree has no branch")
	}
	return PullRequestRequest{
		CLI:         p.forgeCLI,
		WorktreeDir: worktree.Path,
		Branch:      worktree.Branch,
		Title:       title,
		Body:        strings.TrimSpace(p.body.Value()),
	}, nil
}
