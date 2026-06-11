package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Config struct {
	Context           context.Context
	ThemeName         string
	Theme             theme.Styles
	ThemeNames        []string
	Width             int
	Height            int
	Worktrees         []WorktreeState
	SelectedWorktree  int
	Changes           []gitview.FileChange
	Diff              string
	Diffs             map[string]string
	Error             error
	LoadDiff          func(context.Context, string, gitview.FileChange) string
	DeleteWorktree    func(context.Context, gitview.Worktree) error
	Reload            func(context.Context, string) Snapshot
	SaveTheme         func(string) error
	FindForgeCLI      func() (string, bool)
	CreatePullRequest func(context.Context, PullRequestRequest) error
	MergeBranch       func(context.Context, MergeRequest) error
}

type PullRequestRequest struct {
	CLI         string
	WorktreeDir string
	Branch      string
	Title       string
	Body        string
}

type MergeRequest struct {
	Source gitview.Worktree
	Target gitview.Worktree
}

type Snapshot struct {
	Worktrees        []WorktreeState
	SelectedWorktree int
	Changes          []gitview.FileChange
	Diffs            map[string]string
	Error            error
}

type WorktreeState struct {
	Worktree gitview.Worktree
	Changes  []gitview.FileChange
	Error    error
}

const (
	iconFile      = "󰈙"
	iconWorktree  = "󰙅"
	iconBranch    = ""
	iconModified  = ""
	iconAdded     = ""
	iconDeleted   = ""
	iconRenamed   = "󰁕"
	iconUntracked = ""
	iconBinary    = ""
	iconTheme     = ""
	iconQuit      = "󰩈"
	iconKey       = "󰌌"
	iconEdit      = ""
	iconWrap      = "󰖶"
	iconNumbers   = "󰎠"
	iconProtected = ""
	iconPR        = ""
	iconMerge     = ""
	iconStatus    = "󰎟"
	iconSelected  = "▸"
)

const autoRefreshInterval = 5 * time.Second
const toastDuration = 1500 * time.Millisecond

type focusedPane int

const (
	paneWorktrees focusedPane = iota
	paneFiles
	paneDiff
)

type toastKind int

const (
	toastInfo toastKind = iota
	toastError
	toastSuccess
)

type toastState struct {
	Message string
	Kind    toastKind
}

type prFormFocus int

const (
	prFormTitle prFormFocus = iota
	prFormBody
)

type Model struct {
	styles             theme.Styles
	context            context.Context
	themeName          string
	themeNames         []string
	themeCursor        int
	worktrees          []WorktreeState
	selectedWorktree   int
	changes            []gitview.FileChange
	diffs              map[string]string
	diffLines          []string
	selected           int
	worktreeScrollX    int
	fileScrollX        int
	showLineNumbers    bool
	revision           int
	refreshGeneration  int
	width              int
	height             int
	err                error
	toast              toastState
	toastID            int
	pickingTheme       bool
	confirmDelete      bool
	creatingPR         bool
	pickingMergeTarget bool
	prTitle            textinput.Model
	prBody             textarea.Model
	prFormFocus        prFormFocus
	mergeTargetList    list.Model
	mergeSource        gitview.Worktree
	forgeCLI           string
	focusedPane        focusedPane
	loadDiff           func(context.Context, string, gitview.FileChange) string
	deleteWorktree     func(context.Context, gitview.Worktree) error
	reload             func(context.Context, string) Snapshot
	saveTheme          func(string) error
	findForgeCLI       func() (string, bool)
	createPullRequest  func(context.Context, PullRequestRequest) error
	mergeBranch        func(context.Context, MergeRequest) error
	viewport           viewport.Model
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = true
	m := Model{
		styles:            cfg.Theme,
		context:           cfg.Context,
		themeName:         cfg.ThemeName,
		themeNames:        cfg.ThemeNames,
		worktrees:         cfg.Worktrees,
		selectedWorktree:  cfg.SelectedWorktree,
		changes:           cfg.Changes,
		diffs:             cfg.Diffs,
		err:               cfg.Error,
		loadDiff:          cfg.LoadDiff,
		deleteWorktree:    cfg.DeleteWorktree,
		reload:            cfg.Reload,
		saveTheme:         cfg.SaveTheme,
		findForgeCLI:      cfg.FindForgeCLI,
		createPullRequest: cfg.CreatePullRequest,
		mergeBranch:       cfg.MergeBranch,
		viewport:          vp,
		width:             initialDimension(cfg.Width, 100),
		height:            initialDimension(cfg.Height, 30),
		showLineNumbers:   true,
		focusedPane:       paneFiles,
	}
	if m.themeName == "" {
		m.themeName = "tokyonight"
	}
	if m.context == nil {
		m.context = context.Background()
	}
	if m.findForgeCLI == nil {
		m.findForgeCLI = defaultFindForgeCLI
	}
	if m.createPullRequest == nil {
		m.createPullRequest = defaultCreatePullRequest
	}
	if m.mergeBranch == nil {
		m.mergeBranch = defaultMergeBranch
	}
	if len(m.themeNames) == 0 {
		m.themeNames = theme.Names()
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.normalizeWorktrees()
	m.resetPRForm()
	if cfg.Diff != "" && len(cfg.Changes) > 0 {
		m.diffs[m.diffKey(cfg.Changes[0])] = cfg.Diff
	}
	m.refreshDiff()
	return m
}

func initialDimension(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (m Model) Init() tea.Cmd {
	return m.autoRefreshCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case reloadMsg:
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		m.applySnapshot(msg.snapshot)
		return m, m.ensureSelectedDiffCmd()
	case toastExpiredMsg:
		if msg.id == m.toastID {
			m.toast = toastState{}
		}
		return m, nil
	case editorFinishedMsg:
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("editor failed: %s", msg.err))
		}
		return m, nil
	case pullRequestFinishedMsg:
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("PR/MR create failed: %s", msg.err))
		}
		m.creatingPR = false
		return m, m.showSuccessToast("PR/MR created")
	case mergeBranchFinishedMsg:
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("merge failed: %s", msg.err))
		}
		m.pickingMergeTarget = false
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("merged %s into %s", worktreeLabel(msg.request.Source), worktreeLabel(msg.request.Target)))}
		if m.reload != nil {
			m.refreshGeneration++
			cmds = append(cmds, m.reloadCmd(m.refreshGeneration, msg.request.Target.Path))
		}
		return m, tea.Batch(cmds...)
	case deleteWorktreeFinishedMsg:
		m.confirmDelete = false
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("delete failed: %s", msg.err))
		}
		m.removeWorktree(msg.worktree.Path)
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("deleted %s", worktreeLabel(msg.worktree)))}
		if m.reload != nil {
			m.refreshGeneration++
			cmds = append(cmds, m.reloadCmd(m.refreshGeneration, m.SelectedWorktree().Path))
		}
		return m, tea.Batch(cmds...)
	case diffLoadedMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		if msg.path != "" {
			key := msg.worktree + "\x00" + msg.path
			m.diffs[key] = msg.diff
			if selected := m.Selected(); selected.Path == msg.path && m.SelectedWorktree().Path == msg.worktree {
				m.refreshDiff()
			}
		}
		return m, nil
	case autoRefreshMsg:
		m.revision++
		m.refreshGeneration++
		if m.reload == nil {
			return m, m.autoRefreshCmd()
		}
		return m, tea.Batch(
			m.reloadCmd(m.refreshGeneration, m.SelectedWorktree().Path),
			m.autoRefreshCmd(),
		)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
	case tea.MouseClickMsg:
		changed, mouseCmd := m.handleMouse(msg.Mouse())
		if changed {
			return m, tea.Batch(mouseCmd, m.ensureSelectedDiffCmd())
		}
		return m, mouseCmd
	case tea.KeyPressMsg:
		if m.creatingPR {
			return m.handlePRFormKey(msg)
		}
		if m.confirmDelete {
			return m.handleDeleteConfirmKey(msg)
		}
		if m.pickingTheme {
			return m.handleThemeKey(msg)
		}
		if m.pickingMergeTarget {
			return m.handleMergeTargetKey(msg)
		}
		if m.focusPaneShortcut(msg.String()) {
			return m, m.ensureSelectedDiffCmd()
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.focusPreviousPane()
			return m, nil
		case "t":
			m.openThemePicker()
		case "w":
			return m, m.toggleDiffWrap()
		case "n":
			return m, m.toggleLineNumbers()
		case "e":
			return m, m.openSelectedFileInEditor()
		case "d":
			return m, m.openDeleteConfirm()
		case "p":
			return m, m.openPRForm()
		case "m":
			return m, m.openMergeTargetPicker()
		case "tab":
			m.moveWorktree(1)
			return m, m.ensureSelectedDiffCmd()
		case "shift+tab":
			m.moveWorktree(-1)
			return m, m.ensureSelectedDiffCmd()
		case "enter":
			if m.focusedPane == paneWorktrees && len(m.changes) > 0 {
				m.focusedPane = paneFiles
				return m, m.ensureSelectedDiffCmd()
			}
			if m.focusedPane == paneFiles {
				m.focusedPane = paneDiff
				return m, m.ensureSelectedDiffCmd()
			}
		case "j", "down":
			if m.focusedPane == paneWorktrees {
				m.moveWorktree(1)
				return m, m.ensureSelectedDiffCmd()
			}
			if m.focusedPane == paneDiff {
				break
			}
			m.moveSelection(1)
			return m, m.ensureSelectedDiffCmd()
		case "k", "up":
			if m.focusedPane == paneWorktrees {
				m.moveWorktree(-1)
				return m, m.ensureSelectedDiffCmd()
			}
			if m.focusedPane == paneDiff {
				break
			}
			m.moveSelection(-1)
			return m, m.ensureSelectedDiffCmd()
		case "h", "left":
			if m.focusedPane == paneDiff {
				m.scrollDiffHorizontal(-1)
				return m, nil
			}
			if m.scrollFocusedList(-1) {
				return m, nil
			}
			break
		case "l", "right":
			if m.focusedPane == paneDiff {
				m.scrollDiffHorizontal(1)
				return m, nil
			}
			if m.scrollFocusedList(1) {
				return m, nil
			}
			break
		case "g", "home":
			if m.focusedPane == paneDiff {
				m.viewport.GotoTop()
				return m, nil
			}
			m.focusedPane = paneFiles
			m.selected = 0
			m.refreshDiff()
			return m, m.ensureSelectedDiffCmd()
		case "G", "end":
			if m.focusedPane == paneDiff {
				m.viewport.GotoBottom()
				return m, nil
			}
			if len(m.changes) > 0 {
				m.focusedPane = paneFiles
				m.selected = len(m.changes) - 1
				m.refreshDiff()
				return m, m.ensureSelectedDiffCmd()
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	leftWidth, rightWidth := m.layoutWidths()
	contentHeight := m.bodyHeight()

	worktreeHeight := m.worktreePaneHeight(contentHeight)
	worktrees := m.renderWorktrees(leftWidth, worktreeHeight)
	files := m.renderFiles(leftWidth, max(4, contentHeight-lipgloss.Height(worktrees)))
	sidebar := lipgloss.JoinVertical(lipgloss.Left, worktrees, files)
	diff := m.renderDiff(rightWidth, contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, diff)

	footer := m.renderFooter()
	if m.pickingTheme {
		body = m.renderOverlay(body, m.renderThemePicker())
	} else if m.confirmDelete {
		body = m.renderOverlay(body, m.renderDeleteConfirm())
	} else if m.creatingPR {
		body = m.renderOverlay(body, m.renderPRForm())
	} else if m.pickingMergeTarget {
		body = m.renderOverlay(body, m.renderMergeTargetPicker())
	}
	body = m.renderToast(body)

	view := tea.NewView(m.styles.App.Width(m.width).Height(m.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, body, footer),
	))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) footerText() string {
	segments := []string{
		m.footerHint(iconKey, "1/2/3", "panels"),
		m.footerHint(iconWorktree, "tab", "worktree"),
	}
	switch m.focusedPane {
	case paneWorktrees:
		segments = append(segments,
			m.footerHint(iconFile, "hjkl", "move"),
			m.footerHint(iconDeleted, "d", "delete"),
			m.footerHint(iconPR, "p", "PR"),
			m.footerHint(iconMerge, "m", "merge"),
		)
	case paneDiff:
		segments = append(segments,
			m.footerHint(iconFile, "hjkl", "scroll"),
			m.footerHint(iconEdit, "e", "edit"),
			m.footerHint(iconWrap, "w", "wrap"),
			m.footerHint(iconNumbers, "n", "nums"),
		)
	default:
		segments = append(segments,
			m.footerHint(iconFile, "hjkl", "move"),
			m.footerHint(iconEdit, "e", "edit"),
		)
	}
	segments = append(segments,
		m.footerHint(iconTheme, "t", "themes"),
		m.footerHint(iconQuit, "q", "quit"),
	)
	return strings.Join(segments, m.styles.Footer.Render(" │ "))
}

func (m Model) footerHint(icon, key, label string) string {
	keyStyle := m.styles.Footer.Bold(true)
	keyText := keyStyle.Render(key)
	space := m.styles.Footer.Render(" ")
	labelText := m.styles.Footer.Render(label)
	if icon == "" {
		return keyText + space + labelText
	}
	return m.styles.Footer.Render(icon) + space + keyText + space + labelText
}

func (m Model) renderFooter() string {
	style := m.styles.Footer
	text := m.footerText()
	if m.err != nil {
		style = m.styles.Error
		text = m.err.Error()
	}
	width := m.width
	if width <= 0 {
		return style.Render(text)
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return ansi.Cut(text, textWidth-width, textWidth)
	}
	return style.Render(strings.Repeat(" ", width-textWidth)) + text
}

func rightAlignText(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return ansi.Cut(text, textWidth-width, textWidth)
	}
	return strings.Repeat(" ", width-textWidth) + text
}

func defaultFindForgeCLI() (string, bool) {
	for _, name := range []string{"gh", "glab"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, true
		}
	}
	return "", false
}

func defaultCreatePullRequest(ctx context.Context, req PullRequestRequest) error {
	args, err := forgeCreateArgs(req)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, req.CLI, args...)
	if req.WorktreeDir != "" {
		cmd.Dir = req.WorktreeDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("%s %v: %w: %s", req.CLI, args, err, detail)
		}
		return fmt.Errorf("%s %v: %w", req.CLI, args, err)
	}
	return nil
}

func defaultMergeBranch(ctx context.Context, req MergeRequest) error {
	if req.Source.Branch == "" || req.Source.Branch == "detached" {
		return fmt.Errorf("selected worktree has no branch")
	}
	if req.Target.Path == "" {
		return fmt.Errorf("merge target has no worktree path")
	}
	cmd := exec.CommandContext(ctx, "git", "merge", req.Source.Branch)
	cmd.Dir = req.Target.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("git merge %s: %w: %s", req.Source.Branch, err, detail)
		}
		return fmt.Errorf("git merge %s: %w", req.Source.Branch, err)
	}
	return nil
}

func forgeCreateArgs(req PullRequestRequest) ([]string, error) {
	switch req.CLI {
	case "gh":
		return []string{"pr", "create", "--title", req.Title, "--body", req.Body, "--head", req.Branch}, nil
	case "glab":
		return []string{"mr", "create", "--title", req.Title, "--description", req.Body, "--source-branch", req.Branch, "--yes"}, nil
	default:
		return nil, fmt.Errorf("unsupported Forge CLI %q", req.CLI)
	}
}

func (m Model) autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m Model) Selected() gitview.FileChange {
	if len(m.changes) == 0 || m.selected < 0 || m.selected >= len(m.changes) {
		return gitview.FileChange{}
	}
	return m.changes[m.selected]
}

func (m Model) SelectedWorktree() gitview.Worktree {
	if len(m.worktrees) == 0 || m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		return gitview.Worktree{}
	}
	return m.worktrees[m.selectedWorktree].Worktree
}

func (m *Model) moveSelection(delta int) {
	if len(m.changes) == 0 {
		return
	}
	m.focusedPane = paneFiles
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.changes) {
		m.selected = len(m.changes) - 1
	}
	m.refreshDiff()
}

func (m *Model) moveWorktree(delta int) {
	if len(m.worktrees) == 0 {
		return
	}
	m.focusedPane = paneWorktrees
	m.selectedWorktree += delta
	if m.selectedWorktree < 0 {
		m.selectedWorktree = len(m.worktrees) - 1
	}
	if m.selectedWorktree >= len(m.worktrees) {
		m.selectedWorktree = 0
	}
	m.selectWorktree(m.selectedWorktree)
}

func (m *Model) focusPreviousPane() {
	switch m.focusedPane {
	case paneDiff:
		m.focusedPane = paneFiles
	case paneFiles:
		m.focusedPane = paneWorktrees
	}
}

func (m *Model) toggleDiffWrap() tea.Cmd {
	if m.viewport.SoftWrap {
		m.viewport.SoftWrap = false
		return m.showToast("diff wrap off")
	}
	m.viewport.SetXOffset(0)
	m.viewport.SoftWrap = true
	return m.showToast("diff wrap on")
}

func (m *Model) toggleLineNumbers() tea.Cmd {
	m.showLineNumbers = !m.showLineNumbers
	m.viewport.SetXOffset(clamp(m.viewport.XOffset(), 0, m.maxDiffXOffset()))
	if m.showLineNumbers {
		return m.showToast("line numbers on")
	}
	return m.showToast("line numbers off")
}

func (m *Model) openSelectedFileInEditor() tea.Cmd {
	selected := m.Selected()
	if selected.Path == "" {
		return m.showToast("no file selected")
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	worktreePath := m.SelectedWorktree().Path
	if worktreePath == "" {
		worktreePath = "."
	}
	cmd := exec.Command("sh", "-c", `${EDITOR:-vi} "$@"`, "editor", selected.Path)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), "EDITOR="+editor)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m *Model) scrollDiffHorizontal(delta int) {
	if m.viewport.SoftWrap {
		return
	}
	step := 6
	m.viewport.SetXOffset(clamp(m.viewport.XOffset()+delta*step, 0, m.maxDiffXOffset()))
}

func (m Model) maxDiffXOffset() int {
	return max(0, m.maxDiffLineWidth()-m.diffTextWidth(m.viewport.Width()))
}

func (m Model) maxDiffLineWidth() int {
	width := 0
	for _, line := range m.diffLines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func (m *Model) showToast(message string) tea.Cmd {
	return m.showTypedToast(toastInfo, message)
}

func (m *Model) showErrorToast(message string) tea.Cmd {
	return m.showTypedToast(toastError, message)
}

func (m *Model) showSuccessToast(message string) tea.Cmd {
	return m.showTypedToast(toastSuccess, message)
}

func (m *Model) showTypedToast(kind toastKind, message string) tea.Cmd {
	m.toastID++
	m.toast = toastState{Message: message, Kind: kind}
	id := m.toastID
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastExpiredMsg{id: id}
	})
}

func (m *Model) scrollFocusedList(delta int) bool {
	switch m.focusedPane {
	case paneWorktrees:
		m.worktreeScrollX = clamp(m.worktreeScrollX+delta, 0, m.maxWorktreeScrollX())
		return true
	case paneFiles:
		m.fileScrollX = clamp(m.fileScrollX+delta, 0, m.maxFileScrollX())
		return true
	default:
		return false
	}
}

func (m Model) maxWorktreeScrollX() int {
	if len(m.worktrees) == 0 || m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		return 0
	}
	available := m.listContentWidthForPane(paneWorktrees)
	return max(0, lipgloss.Width(renderWorktreeLine(m.styles, m.selectedWorktree, m.worktrees[m.selectedWorktree]))-available)
}

func (m Model) maxFileScrollX() int {
	if len(m.changes) == 0 {
		return 0
	}
	available := m.listContentWidthForPane(paneFiles)
	return max(0, lipgloss.Width(renderFileLine(m.styles, m.Selected()))-available)
}

func (m Model) listContentWidthForPane(_ focusedPane) int {
	leftWidth, _ := m.layoutWidths()
	width := panelInnerWidth(leftWidth)
	return max(1, width-lipgloss.Width(iconSelected+" "))
}

func (m *Model) focusPaneShortcut(key string) bool {
	if len(key) != 1 || key[0] < '1' || key[0] > '3' {
		return false
	}
	m.focusedPane = focusedPane(key[0] - '1')
	return true
}

func (m *Model) openThemePicker() {
	m.pickingTheme = true
	m.themeCursor = indexOf(m.themeNames, m.themeName)
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
}

func (m *Model) openDeleteConfirm() tea.Cmd {
	worktree := m.SelectedWorktree()
	if worktree.Path == "" {
		return m.showErrorToast("no worktree selected")
	}
	if worktree.Protected || gitview.IsProtectedBranch(worktree.Branch) {
		return m.showErrorToast(fmt.Sprintf("protected branch %s cannot be deleted", worktreeLabel(worktree)))
	}
	m.confirmDelete = true
	m.focusedPane = paneWorktrees
	return nil
}

func (m *Model) openPRForm() tea.Cmd {
	cli, ok := m.findForgeCLI()
	if !ok {
		return m.showErrorToast("Forge CLI missing: install gh or glab")
	}
	m.forgeCLI = cli
	m.creatingPR = true
	m.focusedPane = paneWorktrees
	m.resetPRForm()
	return nil
}

func (m *Model) openMergeTargetPicker() tea.Cmd {
	source := m.SelectedWorktree()
	if source.Path == "" {
		return m.showErrorToast("no worktree selected")
	}
	if source.Branch == "" || source.Branch == "detached" {
		return m.showErrorToast("selected worktree has no branch")
	}
	if m.isDefaultBranch(source) {
		return m.showToast("default branch is not a merge source")
	}
	items := m.mergeTargetItems(source)
	if len(items) == 0 {
		return m.showErrorToast("no merge target branch")
	}
	m.mergeSource = source
	m.mergeTargetList = m.newMergeTargetList(items)
	m.mergeTargetList.Select(0)
	m.pickingMergeTarget = true
	m.focusedPane = paneWorktrees
	return nil
}

func (m Model) isDefaultBranch(worktree gitview.Worktree) bool {
	if worktree.DefaultBranch {
		return true
	}
	defaultBranch := m.defaultBranchName()
	if defaultBranch != "" {
		return worktree.Branch == defaultBranch
	}
	return worktree.Branch == "main" || worktree.Branch == "master" || worktree.Branch == "trunk"
}

func (m Model) defaultBranchName() string {
	for _, state := range m.worktrees {
		if state.Worktree.DefaultBranch {
			return state.Worktree.Branch
		}
	}
	for _, candidate := range []string{"main", "master", "trunk"} {
		for _, state := range m.worktrees {
			if state.Worktree.Branch == candidate {
				return candidate
			}
		}
	}
	return ""
}

func (m Model) mergeTargetItems(source gitview.Worktree) []list.Item {
	targets := make([]mergeTargetItem, 0, len(m.worktrees))
	for _, state := range m.worktrees {
		worktree := state.Worktree
		if worktree.Path == "" || worktree.Branch == "" || worktree.Branch == "detached" || worktree.Path == source.Path {
			continue
		}
		targets = append(targets, mergeTargetItem{worktree: worktree})
	}
	defaultBranch := m.defaultBranchName()
	for i, target := range targets {
		if target.worktree.DefaultBranch || target.worktree.Branch == defaultBranch {
			targets[0], targets[i] = targets[i], targets[0]
			break
		}
	}
	items := make([]list.Item, len(targets))
	for i, target := range targets {
		items[i] = target
	}
	return items
}

func (m Model) newMergeTargetList(items []list.Item) list.Model {
	width := min(max(34, m.width-12), 64)
	height := min(max(6, len(items)*2+2), max(6, m.bodyHeight()-4))
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = m.styles.FileItem.Background(m.styles.Panel.GetBackground()).PaddingLeft(2)
	delegate.Styles.NormalDesc = m.styles.Muted.Background(m.styles.Panel.GetBackground()).PaddingLeft(2)
	delegate.Styles.SelectedTitle = m.styles.FileSelected.PaddingLeft(1)
	delegate.Styles.SelectedDesc = m.styles.FileSelected.PaddingLeft(1)
	delegate.Styles.DimmedTitle = delegate.Styles.NormalTitle
	delegate.Styles.DimmedDesc = delegate.Styles.NormalDesc
	delegate.Styles.FilterMatch = m.styles.DiffHunk.Underline(true)

	targets := list.New(items, delegate, width, height)
	targets.Title = iconMerge + " Merge into"
	targets.SetFilteringEnabled(false)
	targets.SetShowFilter(false)
	targets.SetShowStatusBar(false)
	targets.SetShowHelp(false)
	targets.SetShowPagination(false)
	targets.DisableQuitKeybindings()
	targets.Styles.TitleBar = lipgloss.NewStyle().Background(m.styles.Panel.GetBackground())
	targets.Styles.Title = m.styles.Title.Background(m.styles.Panel.GetBackground())
	targets.Styles.NoItems = m.styles.Muted.Background(m.styles.Panel.GetBackground())
	return targets
}

func (m *Model) resetPRForm() {
	m.prTitle = textinput.New()
	m.prTitle.Prompt = ""
	m.prTitle.Placeholder = "PR title"
	m.prTitle.SetWidth(48)
	m.prTitle.Focus()

	m.prBody = textarea.New()
	m.prBody.Prompt = ""
	m.prBody.Placeholder = "PR description"
	m.prBody.ShowLineNumbers = false
	m.prBody.SetWidth(48)
	m.prBody.SetHeight(5)
	m.prBody.Blur()

	m.prFormFocus = prFormTitle
}

func (m Model) handlePRFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+o" || (msg.Code == 'o' && msg.Mod == tea.ModCtrl) {
		return m, m.createPullRequestCmd()
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.creatingPR = false
		return m, nil
	case "tab", "shift+tab":
		m.togglePRFormFocus()
		return m, nil
	}

	var cmd tea.Cmd
	if m.prFormFocus == prFormTitle {
		m.prTitle, cmd = m.prTitle.Update(msg)
		return m, cmd
	}
	m.prBody, cmd = m.prBody.Update(msg)
	return m, cmd
}

func (m *Model) togglePRFormFocus() {
	if m.prFormFocus == prFormTitle {
		m.prTitle.Blur()
		m.prBody.Focus()
		m.prFormFocus = prFormBody
		return
	}
	m.prBody.Blur()
	m.prTitle.Focus()
	m.prFormFocus = prFormTitle
}

func (m *Model) createPullRequestCmd() tea.Cmd {
	req, err := m.pullRequestRequest()
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	return func() tea.Msg {
		return pullRequestFinishedMsg{err: m.createPullRequest(m.context, req)}
	}
}

func (m Model) pullRequestRequest() (PullRequestRequest, error) {
	worktree := m.SelectedWorktree()
	title := strings.TrimSpace(m.prTitle.Value())
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
		CLI:         m.forgeCLI,
		WorktreeDir: worktree.Path,
		Branch:      worktree.Branch,
		Title:       title,
		Body:        strings.TrimSpace(m.prBody.Value()),
	}, nil
}

func (m Model) handleDeleteConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		return m, m.deleteSelectedWorktreeCmd()
	case "n", "N", "esc", "d":
		m.confirmDelete = false
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) deleteSelectedWorktreeCmd() tea.Cmd {
	worktree := m.SelectedWorktree()
	if m.deleteWorktree == nil {
		return func() tea.Msg {
			return deleteWorktreeFinishedMsg{worktree: worktree, err: fmt.Errorf("delete worktree is not configured")}
		}
	}
	return func() tea.Msg {
		return deleteWorktreeFinishedMsg{worktree: worktree, err: m.deleteWorktree(m.context, worktree)}
	}
}

func (m Model) handleThemeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "t":
		m.pickingTheme = false
	case "j", "down":
		if m.themeCursor < len(m.themeNames)-1 {
			m.themeCursor++
		}
	case "k", "up":
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case "enter":
		cmd = m.applyThemeCursor()
		m.pickingTheme = false
	}
	return m, cmd
}

func (m Model) handleMergeTargetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "m":
		m.pickingMergeTarget = false
		return m, nil
	case "enter":
		return m, m.mergeSelectedTargetCmd()
	}
	var cmd tea.Cmd
	m.mergeTargetList, cmd = m.mergeTargetList.Update(msg)
	return m, cmd
}

func (m Model) mergeSelectedTargetCmd() tea.Cmd {
	target, ok := m.mergeTargetList.SelectedItem().(mergeTargetItem)
	if !ok {
		return m.showErrorToast("no merge target branch")
	}
	request := MergeRequest{
		Source: m.mergeSource,
		Target: target.worktree,
	}
	if request.Source.Path == "" {
		request.Source = m.SelectedWorktree()
	}
	return func() tea.Msg {
		return mergeBranchFinishedMsg{request: request, err: m.mergeBranch(m.context, request)}
	}
}

func (m *Model) applyThemeCursor() tea.Cmd {
	if m.themeCursor < 0 || m.themeCursor >= len(m.themeNames) {
		return nil
	}
	name := m.themeNames[m.themeCursor]
	preset, err := theme.Preset(name)
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.themeName = name
	m.styles = theme.NewStyles(preset)
	var cmd tea.Cmd
	if m.saveTheme != nil {
		if err := m.saveTheme(name); err != nil {
			cmd = m.showErrorToast(fmt.Sprintf("Could not save theme: %s", err))
		}
	}
	m.refreshDiff()
	return cmd
}

func (m *Model) handleMouse(mouse tea.Mouse) (bool, tea.Cmd) {
	if m.pickingTheme {
		overlay := m.renderThemePicker()
		x, y := m.overlayPosition(overlay)
		if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) {
			return false, nil
		}
		index := mouse.Y - y - 3
		offset := m.themePickerOffset()
		if index >= 0 && index < m.themePickerVisibleRows() && offset+index < len(m.themeNames) {
			m.themeCursor = offset + index
			cmd := m.applyThemeCursor()
			m.pickingTheme = false
			return false, cmd
		}
		return false, nil
	}
	leftWidth, _ := m.layoutWidths()
	if mouse.X >= leftWidth {
		m.focusedPane = paneDiff
		return false, nil
	}
	if mouse.Y < 2 {
		return false, nil
	}
	contentHeight := m.bodyHeight()
	worktreeHeight := m.worktreePaneHeight(contentHeight)
	bodyY := mouse.Y
	if bodyY >= 0 && bodyY < worktreeHeight {
		index := m.worktreeListOffset(worktreeHeight) + bodyY - 1
		if index >= 0 && index < len(m.worktrees) {
			m.focusedPane = paneWorktrees
			m.selectWorktree(index)
			return true, nil
		}
		return false, nil
	}
	fileY := bodyY - worktreeHeight
	index := m.listOffset(max(4, contentHeight-worktreeHeight)) + fileY - 1
	if index >= 0 && index < len(m.changes) {
		m.focusedPane = paneFiles
		m.selected = index
		m.refreshDiff()
		return true, nil
	}
	return false, nil
}

func (m Model) reloadCmd(generation int, selectedWorktreePath string) tea.Cmd {
	reload := m.reload
	if reload == nil {
		return func() tea.Msg {
			return reloadMsg{generation: generation, snapshot: Snapshot{Changes: m.changes, Diffs: m.diffs, Error: fmt.Errorf("no reload source configured")}}
		}
	}
	return func() tea.Msg {
		return reloadMsg{generation: generation, snapshot: reload(m.context, selectedWorktreePath)}
	}
}

func (m Model) ensureSelectedDiffCmd() tea.Cmd {
	if m.loadDiff == nil {
		return nil
	}
	selected := m.Selected()
	if selected.Path == "" {
		return nil
	}
	if _, ok := m.diffs[m.diffKey(selected)]; ok {
		return nil
	}
	if _, ok := m.diffs[selected.Path]; ok {
		return nil
	}
	worktreePath := m.SelectedWorktree().Path
	return func() tea.Msg {
		return diffLoadedMsg{
			revision: m.revision,
			worktree: worktreePath,
			path:     selected.Path,
			diff:     m.loadDiff(m.context, worktreePath, selected),
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) {
	selected := m.Selected()
	selectedIndex := m.selected
	m.revision++
	m.worktrees = snapshot.Worktrees
	m.selectedWorktree = snapshot.SelectedWorktree
	m.changes = snapshot.Changes
	m.diffs = snapshot.Diffs
	m.err = snapshot.Error
	m.normalizeWorktrees()
	if index := changeIndex(m.changes, selected); index >= 0 {
		m.selected = index
	} else {
		m.selected = min(selectedIndex, max(0, len(m.changes)-1))
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.refreshDiff()
}

func (m *Model) normalizeWorktrees() {
	if len(m.worktrees) == 0 {
		m.worktrees = []WorktreeState{{
			Worktree: gitview.Worktree{Path: ".", Branch: "current", Current: true},
			Changes:  m.changes,
			Error:    m.err,
		}}
		m.selectedWorktree = 0
		return
	}
	if m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		m.selectedWorktree = 0
	}
	m.changes = m.worktrees[m.selectedWorktree].Changes
	if m.worktrees[m.selectedWorktree].Error != nil {
		m.err = m.worktrees[m.selectedWorktree].Error
	}
}

func (m *Model) selectWorktree(index int) {
	if index < 0 || index >= len(m.worktrees) {
		return
	}
	m.selectedWorktree = index
	m.changes = m.worktrees[index].Changes
	m.err = m.worktrees[index].Error
	m.selected = 0
	m.refreshDiff()
}

func (m *Model) removeWorktree(path string) {
	if path == "" {
		return
	}
	for i, state := range m.worktrees {
		if state.Worktree.Path != path {
			continue
		}
		m.worktrees = append(m.worktrees[:i], m.worktrees[i+1:]...)
		if m.selectedWorktree >= len(m.worktrees) {
			m.selectedWorktree = max(0, len(m.worktrees)-1)
		}
		m.normalizeWorktrees()
		m.refreshDiff()
		return
	}
}

func (m *Model) resizeViewport() {
	_, rightWidth := m.layoutWidths()
	contentHeight := m.bodyHeight()
	m.viewport.Style = m.styles.Diff
	m.viewport.SetWidth(max(10, panelInnerWidth(rightWidth)))
	m.viewport.SetHeight(max(3, panelInnerHeight(contentHeight)))
}

func (m *Model) refreshDiff() {
	m.resizeViewport()
	if len(m.changes) == 0 {
		m.setDiffContent("No changes in this worktree.")
		return
	}
	diff := m.diffs[m.diffKey(m.changes[m.selected])]
	if diff == "" {
		diff = m.diffs[m.changes[m.selected].Path]
	}
	if diff == "" {
		diff = fmt.Sprintf("No diff loaded for %s", m.changes[m.selected].Path)
	}
	m.setDiffContent(diff)
	m.viewport.GotoTop()
}

func (m *Model) setDiffContent(diff string) {
	lines := strings.Split(diff, "\n")
	m.diffLines = lines
	m.viewport.StyleLineFunc = func(index int) lipgloss.Style {
		if index < 0 || index >= len(lines) {
			return m.styles.Diff.Inline(true).Width(m.viewport.Width())
		}
		return m.diffLineStyle(lines[index]).Inline(true).Width(m.viewport.Width())
	}
	m.viewport.SetContent(diff)
}

func (m Model) renderWorktrees(width, height int) string {
	focused := m.focusedPane == paneWorktrees
	lines := make([]string, 0, len(m.worktrees))
	contentWidth := panelInnerWidth(width)
	visibleRows := m.worktreeVisibleRows(height)
	offset := m.worktreeListOffset(height)
	end := min(len(m.worktrees), offset+visibleRows)
	for i, worktree := range m.worktrees[offset:end] {
		index := offset + i
		line := renderWorktreeLine(m.styles, index, worktree)
		if index == m.selectedWorktree {
			line = renderScrollableListRow(m.styles.FileSelected, iconSelected+" ", line, m.worktreeScrollX, contentWidth, true)
		} else {
			line = renderScrollableListRow(m.listRowStyle(m.styles.FileItem), m.listFill("  "), line, m.worktreeScrollX, contentWidth, false)
		}
		lines = append(lines, line)
	}
	if end < len(m.worktrees) {
		lines = append(lines, m.styles.Muted.Render(fmt.Sprintf("… %d more", len(m.worktrees)-end)))
	}
	innerHeight := panelInnerHeight(height)
	title := fmt.Sprintf("[1]-%s %d worktrees", iconWorktree, len(m.worktrees))
	return m.renderPanel(width, height, focused, title, strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (m Model) worktreeListOffset(height int) int {
	if len(m.worktrees) == 0 {
		return 0
	}
	visibleRows := m.worktreeVisibleRows(height)
	if m.selectedWorktree < visibleRows {
		return 0
	}
	offset := m.selectedWorktree - visibleRows + 1
	maxOffset := max(0, len(m.worktrees)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) worktreeVisibleRows(height int) int {
	visibleRows := max(1, panelInnerHeight(height))
	if len(m.worktrees) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}

func (m Model) renderFiles(width, height int) string {
	focused := m.focusedPane == paneFiles
	lines := make([]string, 0, len(m.changes))
	contentWidth := panelInnerWidth(width)
	visibleRows := m.fileVisibleRows(height)
	offset := m.listOffset(height)
	end := min(len(m.changes), offset+visibleRows)
	for i, change := range m.changes[offset:end] {
		index := offset + i
		line := renderFileLine(m.styles, change)
		if index == m.selected {
			line = renderScrollableListRow(m.styles.FileSelected, iconSelected+" ", line, m.fileScrollX, contentWidth, true)
		} else {
			line = renderScrollableListRow(m.listRowStyle(m.styles.FileItem), m.listFill("  "), line, m.fileScrollX, contentWidth, false)
		}
		lines = append(lines, line)
	}
	if len(m.changes) == 0 {
		lines = append(lines, m.styles.Muted.Render("No changed files"))
	} else if end < len(m.changes) {
		lines = append(lines, m.styles.Muted.Render(fmt.Sprintf("… %d more", len(m.changes)-end)))
	}
	innerHeight := panelInnerHeight(height)
	title := fmt.Sprintf("[2]-%s %d files", iconFile, len(m.changes))
	return m.renderPanel(width, height, focused, title, strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (m Model) listOffset(height int) int {
	if len(m.changes) == 0 {
		return 0
	}
	visibleRows := m.fileVisibleRows(height)
	if m.selected < visibleRows {
		return 0
	}
	offset := m.selected - visibleRows + 1
	maxOffset := max(0, len(m.changes)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) fileVisibleRows(height int) int {
	visibleRows := max(1, panelInnerHeight(height))
	if len(m.changes) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}

func (m Model) renderDiff(width, height int) string {
	selected := "[3]-" + iconFile + " Diff"
	if change := m.Selected(); change.Path != "" {
		selected = "[3]-" + change.Path
	}
	focused := m.focusedPane == paneDiff
	content := m.renderDiffViewportContent()
	return m.renderPanelWithFillStyles(width, height, focused, selected, content, m.styles.Diff, m.styles.Diff)
}

func (m Model) renderPanel(width, height int, focused bool, title, content string) string {
	return m.renderPanelWithFill(width, height, focused, title, content, m.panelStyle(focused))
}

func (m Model) renderPanelWithFill(width, height int, focused bool, title, content string, fillStyle lipgloss.Style) string {
	return m.renderPanelWithFillStyles(width, height, focused, title, content, fillStyle, fillStyle)
}

func (m Model) renderPanelWithFillStyles(width, height int, focused bool, title, content string, lineFillStyle, emptyFillStyle lipgloss.Style) string {
	width = max(4, width)
	height = max(3, height)
	style := m.panelStyle(focused)
	innerWidth := panelInnerWidth(width)
	innerHeight := panelInnerHeight(height)
	border := style.GetBorderStyle()
	borderStyle := m.panelBorderStyle(style)
	lines := []string{m.renderPanelTop(style, focused, title, width)}
	contentLines := strings.Split(content, "\n")
	for i := range innerHeight {
		fillStyle := lineFillStyle
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		} else {
			fillStyle = emptyFillStyle
		}
		lines = append(lines, m.renderPanelBodyLine(fillStyle, borderStyle, border, line, innerWidth))
	}
	lines = append(lines, m.renderPanelBottom(borderStyle, border, width))
	return strings.Join(fillLines(lines, height), "\n")
}

func (m Model) renderPanelTop(style lipgloss.Style, focused bool, title string, width int) string {
	border := style.GetBorderStyle()
	borderStyle := m.panelBorderStyle(style)
	label := m.renderPanelTitle(style, focused, title)
	innerWidth := panelInnerWidth(width)
	if lipgloss.Width(label)+2 > innerWidth {
		prefix := "  "
		if focused {
			prefix = "● "
		}
		label = m.panelTitleStyle(style, focused).Render(ansi.Truncate(prefix+title, max(1, innerWidth-2), ""))
	}
	titlePad := m.panelFill(style, 1)
	titleSegment := titlePad + label + titlePad
	fillWidth := max(0, innerWidth-lipgloss.Width(titleSegment))
	return borderStyle.Render(border.TopLeft) +
		titleSegment +
		borderStyle.Render(strings.Repeat(border.Top, fillWidth)+border.TopRight)
}

func (m Model) renderPanelBodyLine(style, borderStyle lipgloss.Style, border lipgloss.Border, line string, innerWidth int) string {
	line = ansi.Truncate(line, innerWidth, "")
	padding := m.panelFill(style, max(0, innerWidth-lipgloss.Width(line)))
	return borderStyle.Render(border.Left) + line + padding + borderStyle.Render(border.Right)
}

func (m Model) renderPanelBottom(borderStyle lipgloss.Style, border lipgloss.Border, width int) string {
	innerWidth := panelInnerWidth(width)
	return borderStyle.Render(border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight)
}

func (m Model) renderPanelTitle(style lipgloss.Style, focused bool, text string) string {
	if focused {
		return m.panelTitleStyle(style, focused).Render("● " + text)
	}
	return m.panelTitleStyle(style, focused).Render("  " + text)
}

func (m Model) panelTitleStyle(style lipgloss.Style, focused bool) lipgloss.Style {
	titleStyle := lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBackground())
	if focused {
		titleStyle = titleStyle.Bold(true)
	}
	return titleStyle
}

func (m Model) panelBorderStyle(style lipgloss.Style) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBackground())
}

func (m Model) panelFill(style lipgloss.Style, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(style.GetBackground()).Render(strings.Repeat(" ", width))
}

func (m Model) panelStyle(focused bool) lipgloss.Style {
	if focused {
		return m.styles.PanelFocused
	}
	return m.styles.Panel
}

func (m Model) listRowStyle(style lipgloss.Style) lipgloss.Style {
	return style.Background(m.styles.Panel.GetBackground())
}

func (m Model) listFill(text string) string {
	return listFill(m.styles, text)
}

func (m Model) renderDeleteConfirm() string {
	worktree := m.SelectedWorktree()
	width := 48
	panel := m.overlayPanelStyle()
	lineStyle := m.styles.Diff
	title := m.styles.Error.Background(panel.GetBackground()).Bold(true).Render("DELETE") +
		lineStyle.Bold(true).Width(width-len("DELETE")).Render(" "+worktreeLabel(worktree)+"/")
	yes := m.confirmButton("Y", "es")
	no := m.confirmButton("N", "o")
	options := lipgloss.NewStyle().
		Background(panel.GetBackground()).
		Width(width).
		Align(lipgloss.Center).
		Render(yes + lineStyle.Render("     ") + no)
	lines := []string{
		title,
		lineStyle.Width(width).Render(""),
		lineStyle.Width(width).Align(lipgloss.Center).Render("remove worktree and delete branch"),
		lineStyle.Width(width).Render(""),
		options,
	}
	return panel.Width(width+4).Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (m Model) confirmButton(key, label string) string {
	keyText := m.styles.DiffHunk.Bold(true).Render("[" + key + "]")
	return keyText + m.styles.Diff.Render(label)
}

func (m Model) renderPRForm() string {
	width := min(max(48, m.width-8), 78)
	titleInput := m.prTitle
	bodyInput := m.prBody
	titleInput.SetWidth(max(1, panelInnerWidth(width)))
	bodyInput.SetWidth(max(1, panelInnerWidth(width)))

	bodyPanelHeight := min(10, max(5, m.bodyHeight()-8))
	bodyInput.SetHeight(max(1, panelInnerHeight(bodyPanelHeight)))

	header := m.styles.Title.
		Background(m.overlayPanelStyle().GetBackground()).
		Width(width).
		Render(iconPR + " Forge CLI: " + m.forgeCLI)
	title := m.renderPanel(width, 3, m.prFormFocus == prFormTitle, "PR title", titleInput.View())
	bodyTitle := "PR description    <tab> focus    <c-o> create"
	body := m.renderPanel(width, bodyPanelHeight, m.prFormFocus == prFormBody, bodyTitle, bodyInput.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, title, body)
}

func (m Model) renderThemePicker() string {
	lines := []string{m.styles.Title.Background(m.overlayPanelStyle().GetBackground()).Width(28).Render(iconTheme + " Themes")}
	offset := m.themePickerOffset()
	end := min(len(m.themeNames), offset+m.themePickerVisibleRows())
	for i, name := range m.themeNames[offset:end] {
		index := offset + i
		prefix := "  "
		if index == m.themeCursor {
			prefix = iconSelected + " "
		}
		line := prefix + name
		if index == m.themeCursor {
			line = m.styles.FileSelected.Width(28).Render(line)
		} else {
			line = m.styles.Diff.Width(28).Render(line)
		}
		lines = append(lines, line)
	}
	return m.overlayPanelStyle().Width(34).Render(strings.Join(lines, "\n"))
}

func (m Model) renderMergeTargetPicker() string {
	targets := m.mergeTargetList
	width := min(max(34, m.width-12), 64)
	height := min(max(6, lipgloss.Height(targets.View())), max(6, m.bodyHeight()-4))
	targets.SetSize(width, height)
	return m.overlayPanelStyle().Width(width+4).Padding(1, 2).Render(targets.View())
}

func (m Model) overlayPanelStyle() lipgloss.Style {
	return m.styles.Panel
}

func (m Model) themePickerOffset() int {
	visibleRows := m.themePickerVisibleRows()
	if visibleRows <= 0 || len(m.themeNames) <= visibleRows || m.themeCursor < visibleRows {
		return 0
	}
	return min(m.themeCursor-visibleRows+1, len(m.themeNames)-visibleRows)
}

func (m Model) themePickerVisibleRows() int {
	if len(m.themeNames) == 0 {
		return 0
	}
	available := max(1, m.bodyHeight()-4)
	return min(len(m.themeNames), available)
}

func (m Model) renderToast(background string) string {
	if m.toast.Message == "" {
		return background
	}
	toast := m.renderToastBox()
	x := max(0, m.width-lipgloss.Width(toast)-1)
	y := 1

	bgLines := strings.Split(background, "\n")
	if len(bgLines) == 0 {
		bgLines = []string{""}
	}
	toastLines := strings.Split(toast, "\n")
	for len(bgLines) < y+len(toastLines) {
		bgLines = append(bgLines, "")
	}
	toastWidth := lipgloss.Width(toast)
	for i, toastLine := range toastLines {
		bgIndex := y + i
		line := bgLines[bgIndex]
		left := ansi.Cut(line, 0, x)
		right := ansi.Cut(line, x+toastWidth, lipgloss.Width(line))
		bgLines[bgIndex] = left + toastLine + right
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) renderToastBox() string {
	width := max(8, min(44, m.width-6))
	title, accent := m.toastTitleAndStyle()
	panelBackground := m.styles.Panel.GetBackground()
	titleLine := accent.
		Bold(true).
		Background(panelBackground).
		Width(width).
		Render(iconStatus + " " + title)
	messageLine := m.styles.Diff.
		Background(panelBackground).
		Width(width).
		Render(ansi.Truncate(m.toast.Message, width, "…"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent.GetForeground()).
		Background(panelBackground).
		Padding(0, 1).
		Render(strings.Join([]string{titleLine, messageLine}, "\n"))
}

func (m Model) toastTitleAndStyle() (string, lipgloss.Style) {
	switch m.toast.Kind {
	case toastError:
		return "Error", m.styles.Error
	case toastSuccess:
		return "Success", m.styles.Added
	default:
		return "Info", m.styles.DiffHunk
	}
}

func (m Model) renderOverlay(background, foreground string) string {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	x, y := m.overlayPosition(foreground)

	bgLines := strings.Split(background, "\n")
	fgLines := strings.Split(foreground, "\n")
	if len(bgLines) < y+fgHeight {
		bgLines = append(bgLines, make([]string, y+fgHeight-len(bgLines))...)
	}
	for i, line := range fgLines {
		bgIndex := y + i
		bgLine := bgLines[bgIndex]
		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, x+fgWidth, lipgloss.Width(bgLine))
		bgLines[bgIndex] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) overlayPosition(foreground string) (int, int) {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	bodyWidth := m.width
	bodyHeight := m.bodyHeight()
	x := max(0, (bodyWidth-fgWidth)/2)
	y := max(0, (bodyHeight-fgHeight)/3)
	return x, y
}

func (m Model) renderDiffContent(diff string, width int) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		lines[i] = m.diffLineStyle(line).Width(width).Render(line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDiffViewportContent() string {
	width := m.viewport.Width()
	height := m.viewport.Height()
	if width <= 0 || height <= 0 {
		return ""
	}
	textWidth := m.diffTextWidth(width)
	if m.viewport.SoftWrap {
		return m.renderWrappedDiffViewport(width, textWidth, height)
	}
	return m.renderUnwrappedDiffViewport(width, textWidth, height)
}

func (m Model) renderWrappedDiffViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := m.viewport.YOffset()
	seen := 0
	for _, line := range m.numberedDiffLines() {
		style := m.diffLineStyle(line.text)
		segments := wrapDisplaySegments(line.text, textWidth)
		for segmentIndex, segment := range segments {
			if seen >= offset {
				lines = append(lines, m.renderDiffSegment(style, m.lineNumberGutter(line, segmentIndex > 0), segment, width, textWidth))
				if len(lines) == height {
					return strings.Join(lines, "\n")
				}
			}
			seen++
		}
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth)), "\n")
}

func (m Model) renderUnwrappedDiffViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := m.viewport.YOffset()
	xOffset := m.viewport.XOffset()
	numbered := m.numberedDiffLines()
	for i := offset; i < len(numbered) && len(lines) < height; i++ {
		line := numbered[i]
		segment := ansi.Cut(line.text, xOffset, xOffset+textWidth)
		lines = append(lines, m.renderDiffSegment(m.diffLineStyle(line.text), m.lineNumberGutter(line, false), segment, width, textWidth))
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth)), "\n")
}

func (m Model) renderDiffSegment(style lipgloss.Style, gutter, segment string, width, textWidth int) string {
	if gutter != "" {
		text := style.Inline(true).Width(textWidth).Render(segment)
		return gutter + text
	}
	return style.Inline(true).Width(width).Render(segment)
}

func wrapDisplaySegments(line string, width int) []string {
	width = max(1, width)
	if line == "" {
		return []string{""}
	}
	lineWidth := lipgloss.Width(line)
	segments := make([]string, 0, max(1, (lineWidth+width-1)/width))
	for offset := 0; offset < lineWidth; offset += width {
		segments = append(segments, ansi.Cut(line, offset, offset+width))
	}
	return segments
}

func (m Model) diffTextWidth(width int) int {
	return max(1, width-m.diffGutterWidth())
}

func (m Model) diffGutterWidth() int {
	if !m.showLineNumbers {
		return 0
	}
	return 8
}

type numberedDiffLine struct {
	text string
	old  int
	new  int
}

func (m Model) numberedDiffLines() []numberedDiffLine {
	lines := make([]numberedDiffLine, 0, len(m.diffLines))
	oldLine, newLine := 0, 0
	seenHunkInFile := false
	for _, line := range m.diffLines {
		if strings.HasPrefix(line, "diff --git") {
			seenHunkInFile = false
			lines = append(lines, numberedDiffLine{text: line})
			continue
		}
		if oldStart, newStart, ok := parseDiffHunkHeader(line); ok {
			if seenHunkInFile {
				lines = append(lines, numberedDiffLine{text: ""})
			}
			oldLine = oldStart
			newLine = newStart
			seenHunkInFile = true
			lines = append(lines, numberedDiffLine{text: line})
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines = append(lines, numberedDiffLine{text: line})
		case strings.HasPrefix(line, "+"):
			lines = append(lines, numberedDiffLine{text: line, new: newLine})
			newLine++
		case strings.HasPrefix(line, "-"):
			lines = append(lines, numberedDiffLine{text: line, old: oldLine})
			oldLine++
		case oldLine > 0 || newLine > 0:
			lines = append(lines, numberedDiffLine{text: line, old: oldLine, new: newLine})
			oldLine++
			newLine++
		default:
			lines = append(lines, numberedDiffLine{text: line})
		}
	}
	return lines
}

func parseDiffHunkHeader(line string) (int, int, bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return 0, 0, false
	}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return 0, 0, false
	}
	oldStart, ok := parseHunkStart(parts[1], '-')
	if !ok {
		return 0, 0, false
	}
	newStart, ok := parseHunkStart(parts[2], '+')
	if !ok {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

func parseHunkStart(value string, prefix byte) (int, bool) {
	if len(value) < 2 || value[0] != prefix {
		return 0, false
	}
	value = value[1:]
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = value[:index]
	}
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		parsed = parsed*10 + int(r-'0')
	}
	if parsed <= 0 {
		parsed = 1
	}
	return parsed, true
}

func (m Model) lineNumberGutter(line numberedDiffLine, continuation bool) string {
	if !m.showLineNumbers {
		return ""
	}
	style := m.styles.Muted.Background(m.styles.Diff.GetBackground())
	if continuation {
		return style.Inline(true).Width(m.diffGutterWidth()).Render("")
	}
	return style.Inline(true).Width(m.diffGutterWidth()).Render(fmt.Sprintf("%5s │ ", lineNumberLabel(line)))
}

func lineNumberLabel(line numberedDiffLine) string {
	if line.new > 0 {
		return fmt.Sprintf("%d", line.new)
	}
	if line.old > 0 {
		return fmt.Sprintf("-%d", line.old)
	}
	return ""
}

func lineNumberText(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func fillStyledLines(lines []string, height int, fill string) []string {
	for len(lines) < height {
		lines = append(lines, fill)
	}
	return lines
}

func (m Model) diffLineStyle(line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
		return m.styles.DiffFileHeader
	case strings.HasPrefix(line, "@@"):
		return m.styles.DiffHunk
	case strings.HasPrefix(line, "+"):
		return m.styles.DiffAddition
	case strings.HasPrefix(line, "-"):
		return m.styles.DiffDeletion
	default:
		return m.styles.Diff
	}
}

func (m Model) layoutWidths() (int, int) {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if width < 64 {
		left := max(22, width/3)
		return left, max(24, width-left)
	}
	left := max(28, min(44, width/3))
	right := max(30, width-left)
	return left, right
}

func (m Model) bodyHeight() int {
	return max(4, m.height-1)
}

func (m Model) worktreePaneHeight(contentHeight int) int {
	if len(m.worktrees) <= 1 {
		return 5
	}
	return min(max(6, len(m.worktrees)+4), max(6, contentHeight/3))
}

func (m Model) diffKey(change gitview.FileChange) string {
	return m.SelectedWorktree().Path + "\x00" + change.Path
}

func renderWorktreeLine(styles theme.Styles, _ int, state WorktreeState) string {
	worktree := state.Worktree
	name := worktreeLabel(worktree)
	marker := " "
	if worktree.Current {
		marker = "•"
	}
	if state.Error != nil {
		marker = "!"
	}
	if worktree.Protected {
		marker = iconProtected
	}
	return listStyle(styles, styles.Muted).Render(marker) +
		listFill(styles, " ") +
		listStyle(styles, styles.Muted).Render(iconBranch) +
		listFill(styles, " ") +
		listStyle(styles, styles.FileItem).Render(name)
}

type mergeTargetItem struct {
	worktree gitview.Worktree
}

func (i mergeTargetItem) FilterValue() string {
	return worktreeLabel(i.worktree)
}

func (i mergeTargetItem) Title() string {
	title := iconBranch + " " + worktreeLabel(i.worktree)
	if i.worktree.DefaultBranch {
		title += " (default)"
	}
	return title
}

func (i mergeTargetItem) Description() string {
	if i.worktree.Path == "" {
		return ""
	}
	return i.worktree.Path
}

func listStyle(styles theme.Styles, style lipgloss.Style) lipgloss.Style {
	return style.Background(styles.Panel.GetBackground())
}

func listFill(styles theme.Styles, text string) string {
	return lipgloss.NewStyle().Background(styles.Panel.GetBackground()).Render(text)
}

func worktreeLabel(worktree gitview.Worktree) string {
	if worktree.Branch != "" {
		return worktree.Branch
	}
	return worktree.Path
}

func panelInnerWidth(width int) int {
	return max(1, width-2)
}

func panelInnerHeight(height int) int {
	return max(1, height-2)
}

func fillLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func renderScrollableListRow(style lipgloss.Style, prefix, content string, offset, width int, stripNestedANSI bool) string {
	prefixWidth := lipgloss.Width(prefix)
	contentWidth := max(1, width-prefixWidth)
	offset = max(0, offset)
	content = ansi.Cut(content, offset, offset+contentWidth)
	if stripNestedANSI {
		content = ansi.Strip(content)
	}
	return style.Width(width).Render(prefix + content)
}

func renderFileLine(styles theme.Styles, change gitview.FileChange) string {
	status := statusIcon(change.Status)
	counts := ""
	if change.Binary {
		counts = listFill(styles, " ") +
			listStyle(styles, styles.Muted).Render(iconBinary) +
			listFill(styles, " ") +
			listStyle(styles, styles.Muted).Render("binary")
	} else if change.Additions != 0 || change.Deletions != 0 {
		counts = listFill(styles, " ") +
			listStyle(styles, styles.Added).Render(fmt.Sprintf("+%d", change.Additions)) +
			listFill(styles, " ") +
			listStyle(styles, styles.Deleted).Render(fmt.Sprintf("-%d", change.Deletions))
	}
	return listStyle(styles, styles.Muted).Render(status) +
		listFill(styles, " ") +
		listStyle(styles, styles.FileItem).Render(change.Path) +
		counts
}

func statusIcon(status gitview.ChangeStatus) string {
	switch status {
	case gitview.Added:
		return iconAdded
	case gitview.Modified:
		return iconModified
	case gitview.Deleted:
		return iconDeleted
	case gitview.Renamed:
		return iconRenamed
	case gitview.Untracked:
		return iconUntracked
	default:
		return iconFile
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}

func changeIndex(changes []gitview.FileChange, want gitview.FileChange) int {
	if want.Path == "" {
		return -1
	}
	for i, change := range changes {
		if change.Path == want.Path {
			return i
		}
	}
	return -1
}

type reloadMsg struct {
	generation int
	snapshot   Snapshot
}

type autoRefreshMsg struct{}

type toastExpiredMsg struct {
	id int
}

type editorFinishedMsg struct {
	err error
}

type pullRequestFinishedMsg struct {
	err error
}

type mergeBranchFinishedMsg struct {
	request MergeRequest
	err     error
}

type deleteWorktreeFinishedMsg struct {
	worktree gitview.Worktree
	err      error
}

type diffLoadedMsg struct {
	revision int
	worktree string
	path     string
	diff     string
}
