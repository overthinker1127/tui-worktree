package tui

import (
	"context"
	"fmt"
	"image/color"
	"io"
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
	Transparent       bool
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
	SaveTransparent   func(bool) error
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
	iconToggleOn  = ""
	iconToggleOff = ""
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
	styles              theme.Styles
	context             context.Context
	themeName           string
	transparent         bool
	themeNames          []string
	themeCursor         int
	worktrees           []WorktreeState
	selectedWorktree    int
	changes             []gitview.FileChange
	diffs               map[string]string
	diffLines           []string
	diffContent         string
	diffContentKey      string
	selected            int
	worktreeScrollX     int
	fileScrollX         int
	fileFilter          string
	filteringFiles      bool
	mergeConfirmScrollX int
	showLineNumbers     bool
	revision            int
	refreshGeneration   int
	refreshInFlight     bool
	width               int
	height              int
	err                 error
	toast               toastState
	toastID             int
	pickingTheme        bool
	confirmDelete       bool
	creatingPR          bool
	pickingMergeTarget  bool
	confirmMerge        bool
	submittingPR        bool
	deletingWorktree    bool
	mergingBranch       bool
	prTitle             textinput.Model
	prBody              textarea.Model
	prFormFocus         prFormFocus
	mergeTargetList     list.Model
	mergeSource         gitview.Worktree
	mergeRequest        MergeRequest
	forgeCLI            string
	focusedPane         focusedPane
	loadDiff            func(context.Context, string, gitview.FileChange) string
	deleteWorktree      func(context.Context, gitview.Worktree) error
	reload              func(context.Context, string) Snapshot
	saveTheme           func(string) error
	saveTransparent     func(bool) error
	findForgeCLI        func() (string, bool)
	createPullRequest   func(context.Context, PullRequestRequest) error
	mergeBranch         func(context.Context, MergeRequest) error
	viewport            viewport.Model
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = true
	m := Model{
		styles:            cfg.Theme,
		context:           cfg.Context,
		themeName:         cfg.ThemeName,
		transparent:       cfg.Transparent,
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
		saveTransparent:   cfg.SaveTransparent,
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
		if msg.generation == m.refreshGeneration {
			m.refreshInFlight = false
		}
		if msg.generation != m.refreshGeneration {
			return m, nil
		}
		if m.snapshotUnchanged(msg.snapshot) {
			return m, m.ensureSelectedDiffCmd()
		}
		preservedDiffScroll, diffYOffset, reloadSelectedDiff := m.applySnapshot(msg.snapshot)
		if reloadSelectedDiff {
			return m, m.reloadSelectedDiffCmdWithYOffset(diffYOffset)
		}
		if preservedDiffScroll {
			return m, m.ensureSelectedDiffCmdWithYOffset(diffYOffset)
		}
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
		m.submittingPR = false
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("PR/MR create failed: %s", msg.err))
		}
		m.creatingPR = false
		return m, m.showSuccessToast("PR/MR created")
	case mergeBranchFinishedMsg:
		m.mergingBranch = false
		m.confirmMerge = false
		m.mergeRequest = MergeRequest{}
		m.mergeConfirmScrollX = 0
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("merge failed: %s", msg.err))
		}
		m.pickingMergeTarget = false
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("merged %s into %s", worktreeLabel(msg.request.Source), worktreeLabel(msg.request.Target)))}
		if m.reload != nil {
			cmds = append(cmds, m.startReloadCmd(msg.request.Target.Path))
		}
		return m, tea.Batch(cmds...)
	case deleteWorktreeFinishedMsg:
		m.deletingWorktree = false
		m.confirmDelete = false
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("delete failed: %s", msg.err))
		}
		m.removeWorktree(msg.worktree.Path)
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("deleted %s", worktreeLabel(msg.worktree)))}
		if m.reload != nil {
			cmds = append(cmds, m.startReloadCmd(m.SelectedWorktree().Path))
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
				m.viewport.SetYOffset(msg.diffYOffset)
			}
		}
		return m, nil
	case autoRefreshMsg:
		m.revision++
		if m.reload == nil {
			return m, m.autoRefreshCmd()
		}
		if m.refreshInFlight {
			return m, m.autoRefreshCmd()
		}
		return m, tea.Batch(
			m.startReloadCmd(m.SelectedWorktree().Path),
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
	case tea.MouseWheelMsg:
		if m.pickingTheme {
			return m.handleThemeWheel(msg.Mouse())
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.filteringFiles {
		return m.handleFileFilterKey(msg)
	}
	if m.creatingPR {
		return m.handlePRFormKey(msg)
	}
	if m.confirmDelete {
		return m.handleDeleteConfirmKey(msg)
	}
	if m.confirmMerge {
		return m.handleMergeConfirmKey(msg)
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
		if m.fileFilter != "" {
			selected := m.Selected()
			m.fileFilter = ""
			m.filteringFiles = false
			m.restoreSelectedFile(selected, m.selected)
			return m, m.ensureSelectedDiffCmd()
		}
		m.focusPreviousPane()
		return m, nil
	case "t":
		m.openThemePicker()
	case "w":
		return m, m.toggleDiffWrap()
	case "n":
		return m, m.toggleLineNumbers()
	case "/":
		m.filteringFiles = true
		m.focusedPane = paneFiles
		return m, nil
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
		if m.focusedPane == paneWorktrees && len(m.visibleChanges()) > 0 {
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
	case "0":
		if m.scrollFocusedListToStart() {
			return m, nil
		}
		break
	case "$":
		if m.scrollFocusedListToEnd() {
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
		changes := m.visibleChanges()
		if len(changes) > 0 {
			m.focusedPane = paneFiles
			m.selected = len(changes) - 1
			m.refreshDiff()
			return m, m.ensureSelectedDiffCmd()
		}
	}
	var cmd tea.Cmd
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
	} else if m.filteringFiles {
		body = m.renderOverlay(body, m.renderFileFilter())
	} else if m.confirmDelete {
		body = m.renderOverlay(body, m.renderDeleteConfirm())
	} else if m.creatingPR {
		body = m.renderOverlay(body, m.renderPRForm())
	} else if m.confirmMerge {
		body = m.renderOverlay(body, m.renderMergeConfirm())
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
			m.footerHint(iconFile, "0/$", "edge"),
			m.footerHint(iconFile, "/", "filter"),
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

func (m Model) autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m Model) Selected() gitview.FileChange {
	changes := m.visibleChanges()
	if len(changes) == 0 || m.selected < 0 || m.selected >= len(changes) {
		return gitview.FileChange{}
	}
	return changes[m.selected]
}

func (m Model) visibleChanges() []gitview.FileChange {
	if m.fileFilter == "" {
		return m.changes
	}
	filter := strings.ToLower(m.fileFilter)
	visible := make([]gitview.FileChange, 0, len(m.changes))
	for _, change := range m.changes {
		if strings.Contains(strings.ToLower(change.Path), filter) {
			visible = append(visible, change)
		}
	}
	return visible
}

func (m *Model) restoreSelectedFile(selected gitview.FileChange, fallbackIndex int) {
	changes := m.visibleChanges()
	if index := changeIndex(changes, selected); index >= 0 {
		m.selected = index
	} else {
		m.selected = min(fallbackIndex, max(0, len(changes)-1))
	}
	m.refreshDiff()
}

func (m Model) SelectedWorktree() gitview.Worktree {
	if len(m.worktrees) == 0 || m.selectedWorktree < 0 || m.selectedWorktree >= len(m.worktrees) {
		return gitview.Worktree{}
	}
	return m.worktrees[m.selectedWorktree].Worktree
}

func (m *Model) moveSelection(delta int) {
	changes := m.visibleChanges()
	if len(changes) == 0 {
		return
	}
	m.focusedPane = paneFiles
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(changes) {
		m.selected = len(changes) - 1
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
	line := editorTargetLine(m.selectedDiff())
	cmd := exec.Command("sh", "-c", editorLaunchScript(editor, line), "editor", selected.Path)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), "EDITOR="+editor, fmt.Sprintf("LINE=%d", line))
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

func (m *Model) scrollMergeConfirmHorizontal(delta int) {
	step := 6
	contentWidth := m.mergeConfirmContentWidth(m.mergeConfirmWidth())
	m.mergeConfirmScrollX = clamp(m.mergeConfirmScrollX+delta*step, 0, m.maxMergeConfirmScrollX(contentWidth))
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

func (m *Model) scrollFocusedListToStart() bool {
	switch m.focusedPane {
	case paneWorktrees:
		m.worktreeScrollX = 0
		return true
	case paneFiles:
		m.fileScrollX = 0
		return true
	default:
		return false
	}
}

func (m *Model) scrollFocusedListToEnd() bool {
	switch m.focusedPane {
	case paneWorktrees:
		m.worktreeScrollX = m.maxWorktreeScrollX()
		return true
	case paneFiles:
		m.fileScrollX = m.maxFileScrollX()
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
	return max(0, lipgloss.Width(renderFileLine(m.styles, m.Selected(), m.fileFilter))-available)
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
	themeIndex := indexOf(m.themeNames, m.themeName)
	if themeIndex < 0 {
		m.themeCursor = 1
	} else {
		m.themeCursor = themeIndex + 1
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
	targets := list.New(items, mergeTargetDelegate{styles: m.styles}, width, height)
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

type mergeTargetDelegate struct {
	styles theme.Styles
}

func (d mergeTargetDelegate) Height() int {
	return 2
}

func (d mergeTargetDelegate) Spacing() int {
	return 0
}

func (d mergeTargetDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d mergeTargetDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	target, ok := item.(mergeTargetItem)
	if !ok {
		return
	}

	width := max(1, m.Width())
	titleStyle := d.styles.FileItem.Background(d.styles.Panel.GetBackground())
	descStyle := d.styles.Muted.Background(d.styles.Panel.GetBackground())
	titlePrefix := "  "
	if index == m.Index() {
		titleStyle = d.styles.FileSelected
		descStyle = d.styles.FileSelected
		titlePrefix = iconSelected + " "
	}

	title := renderOverlayLine(titleStyle, width, titlePrefix+target.Title(), 0)
	desc := renderOverlayLine(descStyle, width, "  "+target.Description(), 0)
	_, _ = fmt.Fprintf(w, "%s\n%s", title, desc)
}

func (m *Model) resetPRForm() {
	m.prTitle = textinput.New()
	m.prTitle.Prompt = ""
	m.prTitle.Placeholder = "PR title"
	m.prTitle.SetWidth(48)
	m.prTitle.SetStyles(m.prTextInputStyles(48))
	m.prTitle.Focus()

	m.prBody = textarea.New()
	m.prBody.Prompt = ""
	m.prBody.Placeholder = "PR description"
	m.prBody.ShowLineNumbers = false
	m.prBody.SetWidth(48)
	m.prBody.SetHeight(5)
	m.prBody.SetStyles(m.prTextareaStyles(48, 5))
	m.prBody.Blur()

	m.prFormFocus = prFormTitle
}

func (m Model) prTextInputStyles(width int) textinput.Styles {
	base := m.prInputStyle(m.styles.Diff)
	placeholder := m.prInputStyle(m.styles.Muted)
	prompt := m.prInputStyle(m.styles.DiffHunk)
	if width > 0 {
		base = base.Width(width)
		placeholder = placeholder.Width(width)
	}
	return textinput.Styles{
		Focused: textinput.StyleState{
			Text:        base,
			Placeholder: placeholder,
			Suggestion:  placeholder,
			Prompt:      prompt,
		},
		Blurred: textinput.StyleState{
			Text:        base,
			Placeholder: placeholder,
			Suggestion:  placeholder,
			Prompt:      prompt,
		},
		Cursor: textinput.CursorStyle{
			Color: m.styles.DiffHunk.GetForeground(),
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func (m Model) prTextareaStyles(width, height int) textarea.Styles {
	base := m.prInputStyle(m.styles.Diff)
	placeholder := m.prInputStyle(m.styles.Muted)
	prompt := m.prInputStyle(m.styles.DiffHunk)
	if width > 0 {
		base = base.Width(width)
		placeholder = placeholder.Width(width)
	}
	if height > 0 {
		base = base.Height(height)
	}
	return textarea.Styles{
		Focused: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       placeholder,
			CursorLineNumber: placeholder,
			CursorLine:       base,
			EndOfBuffer:      base,
			Placeholder:      placeholder,
			Prompt:           prompt,
		},
		Blurred: textarea.StyleState{
			Base:             base,
			Text:             base,
			LineNumber:       placeholder,
			CursorLineNumber: placeholder,
			CursorLine:       base,
			EndOfBuffer:      base,
			Placeholder:      placeholder,
			Prompt:           prompt,
		},
		Cursor: textarea.CursorStyle{
			Color: m.styles.DiffHunk.GetForeground(),
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
}

func (m Model) prInputStyle(style lipgloss.Style) lipgloss.Style {
	return style.Background(m.styles.Panel.GetBackground())
}

func (m Model) handlePRFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+o" || (msg.Code == 'o' && msg.Mod == tea.ModCtrl) {
		if m.submittingPR {
			return m, nil
		}
		cmd := m.createPullRequestCmd()
		return m, cmd
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
	m.submittingPR = true
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
		if m.deletingWorktree {
			return m, nil
		}
		m.deletingWorktree = true
		return m, m.deleteSelectedWorktreeCmd()
	case "n", "N", "esc", "d":
		if m.deletingWorktree {
			return m, nil
		}
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
		if m.themeCursor < m.themePickerTotalRows()-1 {
			m.themeCursor++
		}
	case "k", "up":
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case " ", "space":
		if m.themeCursor == 0 {
			cmd = m.toggleTransparentBackground()
		}
	case "enter":
		if m.themeCursor == 0 {
			return m, nil
		} else {
			cmd = m.applyThemeCursor()
		}
		m.pickingTheme = false
	}
	return m, cmd
}

func (m Model) handleMergeTargetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "m":
		if m.mergingBranch {
			return m, nil
		}
		m.pickingMergeTarget = false
		return m, nil
	case "enter":
		if m.mergingBranch {
			return m, nil
		}
		return m, m.openMergeConfirm()
	}
	var cmd tea.Cmd
	m.mergeTargetList, cmd = m.mergeTargetList.Update(msg)
	return m, cmd
}

func (m *Model) openMergeConfirm() tea.Cmd {
	request, ok := m.selectedMergeRequest()
	if !ok {
		return m.showErrorToast("no merge target branch")
	}
	m.mergeRequest = request
	m.mergeConfirmScrollX = 0
	m.pickingMergeTarget = false
	m.confirmMerge = true
	return nil
}

func (m Model) selectedMergeRequest() (MergeRequest, bool) {
	target, ok := m.mergeTargetList.SelectedItem().(mergeTargetItem)
	if !ok {
		return MergeRequest{}, false
	}
	request := MergeRequest{
		Source: m.mergeSource,
		Target: target.worktree,
	}
	if request.Source.Path == "" {
		request.Source = m.SelectedWorktree()
	}
	return request, true
}

func (m Model) handleMergeConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n", "m":
		if m.mergingBranch {
			return m, nil
		}
		m.confirmMerge = false
		m.mergeRequest = MergeRequest{}
		m.mergeConfirmScrollX = 0
		return m, nil
	case "h", "left":
		m.scrollMergeConfirmHorizontal(-1)
		return m, nil
	case "l", "right":
		m.scrollMergeConfirmHorizontal(1)
		return m, nil
	case "enter", "y":
		if m.mergingBranch {
			return m, nil
		}
		return m, m.mergeConfirmedTargetCmd()
	}
	return m, nil
}

func (m *Model) mergeConfirmedTargetCmd() tea.Cmd {
	request := m.mergeRequest
	if request.Source.Path == "" || request.Target.Path == "" {
		return m.showErrorToast("no merge target branch")
	}
	m.mergingBranch = true
	return func() tea.Msg {
		return mergeBranchFinishedMsg{request: request, err: m.mergeBranch(m.context, request)}
	}
}

func (m *Model) applyThemeCursor() tea.Cmd {
	themeIndex := m.themeCursor - 1
	if themeIndex < 0 || themeIndex >= len(m.themeNames) {
		return nil
	}
	name := m.themeNames[themeIndex]
	preset, err := theme.Preset(name)
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.themeName = name
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.transparent})
	var cmd tea.Cmd
	if m.saveTheme != nil {
		if err := m.saveTheme(name); err != nil {
			cmd = m.showErrorToast(fmt.Sprintf("Could not save theme: %s", err))
		}
	}
	m.refreshDiff()
	return cmd
}

func (m *Model) toggleTransparentBackground() tea.Cmd {
	m.transparent = !m.transparent
	preset, err := theme.Preset(m.themeName)
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.transparent})
	m.refreshDiff()
	if m.saveTransparent != nil {
		if err := m.saveTransparent(m.transparent); err != nil {
			return m.showErrorToast(fmt.Sprintf("Could not save transparency: %s", err))
		}
	}
	return nil
}

func (m Model) handleFileFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Code == '\r' || msg.Code == '\n' {
		m.filteringFiles = false
		return m, m.ensureSelectedDiffCmd()
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		selected := m.Selected()
		m.fileFilter = ""
		m.filteringFiles = false
		m.restoreSelectedFile(selected, m.selected)
		return m, m.ensureSelectedDiffCmd()
	case "enter":
		m.filteringFiles = false
		return m, m.ensureSelectedDiffCmd()
	case "backspace":
		if m.fileFilter == "" {
			return m, nil
		}
		selected := m.Selected()
		runes := []rune(m.fileFilter)
		m.fileFilter = string(runes[:len(runes)-1])
		m.restoreSelectedFile(selected, m.selected)
		return m, m.ensureSelectedDiffCmd()
	default:
		text := msg.Text
		if text == "" {
			text = msg.String()
		}
		runes := []rune(text)
		if len(runes) != 1 || runes[0] < 0x20 || runes[0] == 0x7f {
			return m, nil
		}
		selected := m.Selected()
		m.fileFilter += text
		m.restoreSelectedFile(selected, 0)
		return m, m.ensureSelectedDiffCmd()
	}
}

func (m *Model) handleMouse(mouse tea.Mouse) (bool, tea.Cmd) {
	if m.pickingTheme {
		overlay := m.renderThemePicker()
		x, y := m.overlayPosition(overlay)
		if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) {
			return false, nil
		}
		index := mouse.Y - y - 2
		if index == 0 {
			m.themeCursor = 0
			cmd := m.toggleTransparentBackground()
			return false, cmd
		}
		offset := m.themePickerOffset()
		if index > 0 && index <= m.themePickerVisibleThemeRows() && offset+index-1 < len(m.themeNames) {
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
	if mouse.Y < 1 {
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
	changes := m.visibleChanges()
	index := m.listOffset(max(4, contentHeight-worktreeHeight)) + fileY - 1
	if index >= 0 && index < len(changes) {
		m.focusedPane = paneFiles
		m.selected = index
		m.refreshDiff()
		return true, nil
	}
	return false, nil
}

func (m Model) handleThemeWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	overlay := m.renderThemePicker()
	x, y := m.overlayPosition(overlay)
	if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) || mouse.Y < y || mouse.Y >= y+lipgloss.Height(overlay) {
		return m, nil
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case tea.MouseWheelDown:
		if m.themeCursor < m.themePickerTotalRows()-1 {
			m.themeCursor++
		}
	}
	return m, nil
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

func (m *Model) startReloadCmd(selectedWorktreePath string) tea.Cmd {
	m.refreshGeneration++
	m.refreshInFlight = true
	return m.reloadCmd(m.refreshGeneration, selectedWorktreePath)
}

func (m Model) ensureSelectedDiffCmd() tea.Cmd {
	return m.ensureSelectedDiffCmdWithYOffset(m.viewport.YOffset())
}

func (m Model) ensureSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, false)
}

func (m Model) reloadSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, true)
}

func (m Model) selectedDiffCmdWithYOffset(diffYOffset int, force bool) tea.Cmd {
	if m.loadDiff == nil {
		return nil
	}
	selected := m.Selected()
	if selected.Path == "" {
		return nil
	}
	if !force {
		if _, ok := m.diffs[m.diffKey(selected)]; ok {
			return nil
		}
		if _, ok := m.diffs[selected.Path]; ok {
			return nil
		}
	}
	worktreePath := m.SelectedWorktree().Path
	return func() tea.Msg {
		return diffLoadedMsg{
			revision:    m.revision,
			worktree:    worktreePath,
			path:        selected.Path,
			diff:        m.loadDiff(m.context, worktreePath, selected),
			diffYOffset: diffYOffset,
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) (bool, int, bool) {
	selected := m.Selected()
	selectedWorktreePath := m.SelectedWorktree().Path
	selectedIndex := m.selected
	diffYOffset := m.viewport.YOffset()
	preserveMergeTargetPicker := m.pickingMergeTarget
	mergeSource := m.mergeSource
	mergeTargetPath := ""
	if target, ok := m.mergeTargetList.SelectedItem().(mergeTargetItem); ok {
		mergeTargetPath = target.worktree.Path
	}
	mergeRequest := m.mergeRequest
	preserveMergeConfirm := m.confirmMerge
	preserveMergingBranch := m.mergingBranch
	preserveMergeConfirmScrollX := m.mergeConfirmScrollX
	m.pickingMergeTarget = false
	m.mergingBranch = false
	m.mergeSource = gitview.Worktree{}
	m.confirmMerge = false
	m.mergeRequest = MergeRequest{}
	m.mergeConfirmScrollX = 0
	m.revision++
	m.worktrees = snapshot.Worktrees
	m.selectedWorktree = snapshot.SelectedWorktree
	m.changes = snapshot.Changes
	if snapshot.Diffs != nil {
		m.diffs = snapshot.Diffs
	}
	m.err = snapshot.Error
	m.normalizeWorktrees()
	if preserveMergeConfirm {
		if request, ok := m.refreshedMergeRequest(mergeRequest); ok {
			m.confirmMerge = true
			m.mergingBranch = preserveMergingBranch
			m.mergeRequest = request
			m.mergeSource = request.Source
			m.mergeConfirmScrollX = clamp(preserveMergeConfirmScrollX, 0, m.maxMergeConfirmScrollX(m.mergeConfirmContentWidth(m.mergeConfirmWidth())))
		}
	}
	if !m.confirmMerge && preserveMergeTargetPicker {
		m.restoreMergeTargetPicker(mergeSource, mergeTargetPath)
	}
	visibleChanges := m.visibleChanges()
	if index := changeIndex(visibleChanges, selected); index >= 0 {
		m.selected = index
	} else {
		m.selected = min(selectedIndex, max(0, len(visibleChanges)-1))
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.refreshDiff()
	preservedDiffScroll := selected.Path != "" && m.Selected().Path == selected.Path && m.SelectedWorktree().Path == selectedWorktreePath
	reloadSelectedDiff := snapshot.Diffs == nil && preservedDiffScroll && selectedDiffStatsChanged(selected, m.Selected())
	if preservedDiffScroll {
		m.viewport.SetYOffset(diffYOffset)
	}
	return preservedDiffScroll, diffYOffset, reloadSelectedDiff
}

func (m Model) snapshotUnchanged(snapshot Snapshot) bool {
	return m.selectedWorktree == snapshot.SelectedWorktree &&
		errorText(m.err) == errorText(snapshot.Error) &&
		fileChangesEqual(m.changes, snapshot.Changes) &&
		(snapshot.Diffs == nil || diffsEqual(m.diffs, snapshot.Diffs)) &&
		worktreeStatesEqual(m.worktrees, snapshot.Worktrees)
}

func selectedDiffStatsChanged(previous, next gitview.FileChange) bool {
	return previous.Status != next.Status ||
		previous.OldPath != next.OldPath ||
		previous.Additions != next.Additions ||
		previous.Deletions != next.Deletions ||
		previous.Binary != next.Binary
}

func worktreeStatesEqual(left, right []WorktreeState) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Worktree != right[i].Worktree || !fileChangesEqual(left[i].Changes, right[i].Changes) || errorText(left[i].Error) != errorText(right[i].Error) {
			return false
		}
	}
	return true
}

func fileChangesEqual(left, right []gitview.FileChange) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func diffsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if rightValue, ok := right[key]; !ok || rightValue != leftValue {
			return false
		}
	}
	return true
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (m *Model) restoreMergeTargetPicker(source gitview.Worktree, selectedTargetPath string) bool {
	refreshedSource, ok := m.worktreeByPath(source.Path)
	if !ok || refreshedSource.Branch == "" || refreshedSource.Branch == "detached" || m.isDefaultBranch(refreshedSource) {
		return false
	}
	items := m.mergeTargetItems(refreshedSource)
	if len(items) == 0 {
		return false
	}
	m.mergeSource = refreshedSource
	m.mergeTargetList = m.newMergeTargetList(items)
	if index := mergeTargetItemIndex(items, selectedTargetPath); index >= 0 {
		m.mergeTargetList.Select(index)
	}
	m.pickingMergeTarget = true
	m.focusedPane = paneWorktrees
	return true
}

func mergeTargetItemIndex(items []list.Item, path string) int {
	if path == "" {
		return -1
	}
	for i, item := range items {
		target, ok := item.(mergeTargetItem)
		if ok && target.worktree.Path == path {
			return i
		}
	}
	return -1
}

func (m Model) refreshedMergeRequest(request MergeRequest) (MergeRequest, bool) {
	source, sourceOK := m.worktreeByPath(request.Source.Path)
	target, targetOK := m.worktreeByPath(request.Target.Path)
	if !sourceOK || !targetOK {
		return MergeRequest{}, false
	}
	return MergeRequest{
		Source: source,
		Target: target,
	}, true
}

func (m Model) worktreeByPath(path string) (gitview.Worktree, bool) {
	if path == "" {
		return gitview.Worktree{}, false
	}
	for _, state := range m.worktrees {
		if state.Worktree.Path == path {
			return state.Worktree, true
		}
	}
	return gitview.Worktree{}, false
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
	changes := m.visibleChanges()
	if len(changes) == 0 {
		if len(m.changes) > 0 && m.fileFilter != "" {
			m.setDiffContentFor("empty:no-matching:"+m.fileFilter, "No matching files.")
			return
		}
		m.setDiffContentFor("empty:no-changes:"+m.SelectedWorktree().Path, "No changes in this worktree.")
		return
	}
	m.selected = clamp(m.selected, 0, len(changes)-1)
	selected := changes[m.selected]
	diffContentKey := "diff:" + m.diffKey(selected)
	diff := m.selectedDiff()
	if diff == "" {
		diff = fmt.Sprintf("No diff loaded for %s", selected.Path)
	}
	if m.setDiffContentFor(diffContentKey, diff) {
		m.viewport.GotoTop()
	}
}

func (m Model) selectedDiff() string {
	selected := m.Selected()
	if selected.Path == "" {
		return ""
	}
	diff := m.diffs[m.diffKey(selected)]
	if diff == "" {
		diff = m.diffs[selected.Path]
	}
	return diff
}

func (m *Model) setDiffContent(diff string) bool {
	return m.setDiffContentFor("", diff)
}

func (m *Model) setDiffContentFor(key, diff string) bool {
	if key == m.diffContentKey && diff == m.diffContent {
		return false
	}
	lines := strings.Split(diff, "\n")
	m.diffContentKey = key
	m.diffContent = diff
	m.diffLines = lines
	m.viewport.StyleLineFunc = func(index int) lipgloss.Style {
		if index < 0 || index >= len(lines) {
			return m.styles.Diff.Inline(true).Width(m.viewport.Width())
		}
		return m.diffLineStyle(lines[index]).Inline(true).Width(m.viewport.Width())
	}
	m.viewport.SetContent(diff)
	return true
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
	changes := m.visibleChanges()
	lines := make([]string, 0, len(changes))
	contentWidth := panelInnerWidth(width)
	visibleRows := m.fileVisibleRows(height)
	offset := m.listOffset(height)
	end := min(len(changes), offset+visibleRows)
	for i, change := range changes[offset:end] {
		index := offset + i
		if index == m.selected {
			line := m.renderFileListLine(change, m.styles.FileSelected.GetBackground(), contentWidth)
			line = renderScrollableListRow(m.styles.FileSelected, iconSelected+" ", line, m.fileScrollX, contentWidth, false)
			lines = append(lines, line)
		} else {
			line := m.renderFileListLine(change, m.styles.Panel.GetBackground(), contentWidth)
			line = renderScrollableListRow(m.listRowStyle(m.styles.FileItem), m.listFill("  "), line, m.fileScrollX, contentWidth, false)
			lines = append(lines, line)
		}
	}
	if len(m.changes) == 0 {
		lines = append(lines, m.listRowStyle(m.styles.Muted).Render("No changed files"))
	} else if len(changes) == 0 {
		lines = append(lines, m.listRowStyle(m.styles.Muted).Render("No matching files"))
	} else if end < len(changes) {
		lines = append(lines, m.listRowStyle(m.styles.Muted).Render(fmt.Sprintf("… %d more", len(changes)-end)))
	}
	innerHeight := panelInnerHeight(height)
	title := m.filesTitle(len(changes))
	return m.renderPanel(width, height, focused, title, strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (m Model) renderFileListLine(change gitview.FileChange, background color.Color, rowWidth int) string {
	if m.fileScrollX > 0 || m.fileFilter != "" {
		return renderFileLineWithBackground(m.styles, change, m.fileFilter, background)
	}
	prefixWidth := lipgloss.Width(iconSelected + " ")
	contentWidth := max(1, rowWidth-prefixWidth)
	return renderFileLineWithinWidth(m.styles, change, m.fileFilter, background, contentWidth)
}

func (m Model) listOffset(height int) int {
	changes := m.visibleChanges()
	if len(changes) == 0 {
		return 0
	}
	visibleRows := m.fileVisibleRows(height)
	if m.selected < visibleRows {
		return 0
	}
	offset := m.selected - visibleRows + 1
	maxOffset := max(0, len(changes)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) fileVisibleRows(height int) int {
	changes := m.visibleChanges()
	visibleRows := max(1, panelInnerHeight(height))
	if len(changes) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}

func (m Model) filesTitle(visibleCount int) string {
	if m.fileFilter != "" {
		return fmt.Sprintf("[2]-%s %d filtered [Esc]", iconFile, visibleCount)
	}
	title := fmt.Sprintf("[2]-%s %d files", iconFile, visibleCount)
	return title
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
	inputWidth := max(1, panelInnerWidth(width))
	titleInput.SetWidth(inputWidth)
	titleInput.SetStyles(m.prTextInputStyles(inputWidth))
	bodyInput.SetWidth(inputWidth)

	bodyPanelHeight := min(10, max(5, m.bodyHeight()-8))
	bodyInputHeight := max(1, panelInnerHeight(bodyPanelHeight))
	bodyInput.SetHeight(bodyInputHeight)
	bodyInput.SetStyles(m.prTextareaStyles(inputWidth, bodyInputHeight))

	header := m.styles.Title.
		Background(m.overlayPanelStyle().GetBackground()).
		Width(width).
		Render(iconPR + " Forge CLI: " + m.forgeCLI)
	title := m.renderPanel(width, 3, m.prFormFocus == prFormTitle, "PR title", titleInput.View())
	bodyTitle := "PR description    <tab> focus    <c-o> create"
	body := m.renderPanel(width, bodyPanelHeight, m.prFormFocus == prFormBody, bodyTitle, bodyInput.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, title, body)
}

func (m Model) renderFileFilter() string {
	width := min(max(32, m.width-20), 56)
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	title := m.styles.Title.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(iconFile + " Filters")
	query := m.styles.Diff.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(m.fileFilter)
	return panel.Render(lipgloss.JoinVertical(lipgloss.Left, title, query))
}

func (m Model) renderThemePicker() string {
	width := m.themePickerWidth()
	lines := []string{m.styles.Title.Background(m.overlayPanelStyle().GetBackground()).Width(width).Render(iconTheme + " Themes")}
	lines = append(lines, m.renderThemePickerRow(0, width))
	offset := m.themePickerOffset()
	end := min(len(m.themeNames), offset+m.themePickerVisibleThemeRows())
	for themeIndex := offset; themeIndex < end; themeIndex++ {
		lines = append(lines, m.renderThemePickerRow(themeIndex+1, width))
	}
	contentRows := max(3, m.themePickerOverlayHeight()-2)
	for len(lines) < contentRows-1 {
		lines = append(lines, m.styles.Diff.Width(width).Render(""))
	}
	lines = append(lines, m.renderThemePickerFooter(width))
	return m.overlayPanelStyle().Width(width + 6).Render(strings.Join(lines, "\n"))
}

func (m Model) themePickerWidth() int {
	available := max(28, m.width-8)
	target := max(34, (m.width*2)/3)
	return min(target, available)
}

func (m Model) renderThemePickerRow(index, width int) string {
	prefix := "  "
	if index == m.themeCursor {
		prefix = iconSelected + " "
	}
	line := prefix + m.themePickerRowLabel(index)
	if index == m.themeCursor {
		return m.styles.FileSelected.Width(width).Render(line)
	}
	return m.styles.Diff.Width(width).Render(line)
}

func (m Model) themePickerRowLabel(index int) string {
	if index == 0 {
		state := iconToggleOff
		if m.transparent {
			state = iconToggleOn
		}
		return "Transparent background  " + state
	}
	themeIndex := index - 1
	if themeIndex < 0 || themeIndex >= len(m.themeNames) {
		return ""
	}
	return m.themeNames[themeIndex]
}

func (m Model) renderThemePickerFooter(width int) string {
	label := m.themePickerPositionLabel()
	if m.themeCursor == 0 {
		label = m.themePickerFooterLabel("space toggle", label, width)
	}
	return m.styles.Muted.
		Background(m.overlayPanelStyle().GetBackground()).
		Width(width).
		Align(lipgloss.Right).
		Render(label)
}

func (m Model) themePickerFooterLabel(hint, position string, width int) string {
	if hint == "" {
		return position
	}
	if position == "" {
		return hint
	}
	spaceWidth := width - ansi.StringWidth(hint) - ansi.StringWidth(position)
	if spaceWidth < 2 {
		return hint
	}
	return hint + strings.Repeat(" ", spaceWidth) + position
}

func (m Model) themePickerPositionLabel() string {
	total := len(m.themeNames)
	if total == 0 {
		return "0/0"
	}
	current := clamp(m.themeCursor, 0, total)
	return fmt.Sprintf("%d/%d", current, total)
}

func (m Model) renderMergeTargetPicker() string {
	targets := m.mergeTargetList
	width := min(max(40, m.width-12), 70)
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	height := min(max(6, len(targets.Items())*2+2), max(6, m.bodyHeight()-5))
	targets.SetSize(contentWidth, height)
	source := renderOverlayLine(m.styles.Muted, contentWidth, "From: "+worktreeLabel(m.mergeSource)+"  >  Target: "+selectedMergeTargetLabel(targets), 0)
	return panel.Render(lipgloss.JoinVertical(lipgloss.Left, source, targets.View()))
}

func selectedMergeTargetLabel(targets list.Model) string {
	target, ok := targets.SelectedItem().(mergeTargetItem)
	if !ok {
		return ""
	}
	return worktreeLabel(target.worktree)
}

func (m Model) renderMergeConfirm() string {
	width := m.mergeConfirmWidth()
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	scrollX := clamp(m.mergeConfirmScrollX, 0, m.maxMergeConfirmScrollX(contentWidth))
	texts := m.mergeConfirmTextLines()
	lines := []string{
		renderOverlayLine(m.styles.Title.Background(panel.GetBackground()), contentWidth, texts[0], scrollX),
		renderOverlayLine(m.styles.Diff, contentWidth, texts[1], scrollX),
		renderOverlayLine(m.styles.Diff, contentWidth, texts[2], scrollX),
		"",
		renderOverlayLine(m.styles.Muted, contentWidth, texts[3], 0),
		renderOverlayLine(m.styles.Muted, contentWidth, texts[4], 0),
		"",
		renderOverlayLine(m.styles.FileSelected, contentWidth, texts[5], 0),
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m Model) mergeConfirmTextLines() []string {
	request := m.mergeRequest
	source := worktreeLabel(request.Source)
	target := worktreeLabel(request.Target)
	return []string{
		iconMerge + " Merge " + source + " into " + target,
		"Source: " + source,
		"Target: " + target,
		"Target worktree will be updated first.",
		"Dirty files and conflicts will be checked before merging.",
		"[Enter] merge  [Y]es  [Esc] cancel  [N]o",
	}
}

func (m Model) mergeConfirmWidth() int {
	return min(max(50, m.width-12), 72)
}

func (m Model) mergeConfirmContentWidth(width int) int {
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	return max(1, width-panel.GetHorizontalFrameSize())
}

func (m Model) maxMergeConfirmScrollX(contentWidth int) int {
	lineWidth := 0
	for _, line := range m.mergeConfirmTextLines()[:3] {
		lineWidth = max(lineWidth, ansi.StringWidth(line))
	}
	return max(0, lineWidth-contentWidth)
}

func renderOverlayLine(style lipgloss.Style, width int, text string, offset int) string {
	if width <= 0 {
		return ""
	}
	offset = max(0, offset)
	rendered := style.Inline(true).MaxWidth(width).Render(ansi.Cut(text, offset, offset+width))
	if renderedWidth := lipgloss.Width(rendered); renderedWidth < width {
		rendered += style.Inline(true).Render(strings.Repeat(" ", width-renderedWidth))
	}
	return rendered
}

func (m Model) overlayPanelStyle() lipgloss.Style {
	return m.styles.Panel
}

func (m Model) themePickerOffset() int {
	visibleRows := m.themePickerVisibleThemeRows()
	themeCursor := max(0, m.themeCursor-1)
	if visibleRows <= 0 || len(m.themeNames) <= visibleRows || themeCursor < visibleRows {
		return 0
	}
	return min(themeCursor-visibleRows+1, len(m.themeNames)-visibleRows)
}

func (m Model) themePickerVisibleRows() int {
	if m.themePickerTotalRows() == 0 {
		return 0
	}
	contentRows := max(2, m.themePickerOverlayHeight()-2)
	available := max(1, contentRows-2)
	return min(m.themePickerTotalRows(), available)
}

func (m Model) themePickerVisibleThemeRows() int {
	return max(0, m.themePickerVisibleRows()-1)
}

func (m Model) themePickerTotalRows() int {
	return len(m.themeNames) + 1
}

func (m Model) themePickerOverlayHeight() int {
	bodyHeight := max(6, m.bodyHeight())
	return max(6, (bodyHeight*2)/3)
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
	panel := m.overlayPanelStyle()
	panelBackground := panel.GetBackground()
	titleLine := accent.
		Bold(true).
		Background(panelBackground).
		Width(width).
		Render(iconStatus + " " + title)
	messageLine := m.styles.Diff.
		Background(panelBackground).
		Width(width).
		Render(ansi.Truncate(m.toast.Message, width, "…"))
	return panel.
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

func listStyleWithBackground(style lipgloss.Style, background color.Color) lipgloss.Style {
	return style.Background(background)
}

func listFill(styles theme.Styles, text string) string {
	return listFillWithBackground(styles.Panel.GetBackground(), text)
}

func listFillWithBackground(background color.Color, text string) string {
	return lipgloss.NewStyle().Background(background).Render(text)
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

func renderFileLine(styles theme.Styles, change gitview.FileChange, filter string) string {
	return renderFileLineWithBackground(styles, change, filter, styles.Panel.GetBackground())
}

func renderFileLineWithBackground(styles theme.Styles, change gitview.FileChange, filter string, background color.Color) string {
	status := statusIcon(change.Status)
	return listStyleWithBackground(styles.Muted, background).Render(status) +
		listFillWithBackground(background, " ") +
		renderFilteredPathWithBackground(styles, change.Path, filter, background) +
		fileLineCounts(styles, change, background)
}

func renderFileLineWithinWidth(styles theme.Styles, change gitview.FileChange, filter string, background color.Color, width int) string {
	status := listStyleWithBackground(styles.Muted, background).Render(statusIcon(change.Status))
	space := listFillWithBackground(background, " ")
	counts := fileLineCounts(styles, change, background)
	pathWidth := max(0, width-lipgloss.Width(status)-lipgloss.Width(space)-lipgloss.Width(counts))
	path := middleEllipsizePath(change.Path, pathWidth)
	return status + space + renderFilteredPathWithBackground(styles, path, filter, background) + counts
}

func fileLineCounts(styles theme.Styles, change gitview.FileChange, background color.Color) string {
	if change.Binary {
		return listFillWithBackground(background, " ") +
			listStyleWithBackground(styles.Muted, background).Render(iconBinary) +
			listFillWithBackground(background, " ") +
			listStyleWithBackground(styles.Muted, background).Render("binary")
	}
	if change.Additions == 0 && change.Deletions == 0 {
		return ""
	}
	return listFillWithBackground(background, " ") +
		listStyleWithBackground(styles.Added, background).Render(fmt.Sprintf("+%d", change.Additions)) +
		listFillWithBackground(background, " ") +
		listStyleWithBackground(styles.Deleted, background).Render(fmt.Sprintf("-%d", change.Deletions))
}

func middleEllipsizePath(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= width {
		return path
	}
	if width == 1 {
		return "…"
	}
	available := width - 1
	prefixWidth := max(1, available/3)
	suffixWidth := max(0, available-prefixWidth)
	if suffixWidth == 0 {
		return ansi.Cut(path, 0, prefixWidth) + "…"
	}
	return ansi.Cut(path, 0, prefixWidth) + "…" + ansi.Cut(path, lipgloss.Width(path)-suffixWidth, lipgloss.Width(path))
}

func renderFilteredPath(styles theme.Styles, path, filter string) string {
	return renderFilteredPathWithBackground(styles, path, filter, styles.Panel.GetBackground())
}

func renderFilteredPathWithBackground(styles theme.Styles, path, filter string, background color.Color) string {
	if filter == "" {
		return listStyleWithBackground(styles.FileItem, background).Render(path)
	}
	index := strings.Index(strings.ToLower(path), strings.ToLower(filter))
	if index < 0 {
		return listStyleWithBackground(styles.FileItem, background).Render(path)
	}
	end := index + len(filter)
	base := listStyleWithBackground(styles.FileItem, background)
	match := base.Bold(true)
	return base.Render(path[:index]) + match.Render(path[index:end]) + base.Render(path[end:])
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
	revision    int
	worktree    string
	path        string
	diff        string
	diffYOffset int
}
