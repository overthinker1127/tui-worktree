package tui

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/command"
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

type PullRequestRequest = command.PullRequestRequest

type MergeRequest = command.MergeRequest

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

type prFocus int

const (
	prTitle prFocus = iota
	prBody
)

type Model struct {
	styles            theme.Styles
	context           context.Context
	changes           []gitview.FileChange
	diffs             map[string]string
	selected          int
	fileScrollX       int
	fileFilter        string
	revision          int
	refreshGeneration int
	refreshInFlight   bool
	width             int
	height            int
	err               error
	toast             toastState
	toastID           int
	mode              uiMode
	deletingWorktree  bool
	focusedPane       focusedPane
	diff              diffView
	pr                pr
	merge             merge
	overlap           overlap
	themePicker       themePicker
	worktreeList
	deps dependencies
}

func NewModel(cfg Config) Model {
	vp := viewport.New()
	vp.SoftWrap = true
	m := Model{
		styles:       cfg.Theme,
		context:      cfg.Context,
		themePicker:  themePicker{name: cfg.ThemeName, transparent: cfg.Transparent, names: cfg.ThemeNames},
		worktreeList: worktreeList{worktrees: cfg.Worktrees, selectedWorktree: cfg.SelectedWorktree},
		changes:      cfg.Changes,
		diffs:        cfg.Diffs,
		err:          cfg.Error,
		deps: dependencies{
			loadDiff:          cfg.LoadDiff,
			deleteWorktree:    cfg.DeleteWorktree,
			reload:            cfg.Reload,
			saveTheme:         cfg.SaveTheme,
			saveTransparent:   cfg.SaveTransparent,
			findForgeCLI:      cfg.FindForgeCLI,
			createPullRequest: cfg.CreatePullRequest,
			mergeBranch:       cfg.MergeBranch,
		},
		width:       initialDimension(cfg.Width, 100),
		height:      initialDimension(cfg.Height, 30),
		focusedPane: paneFiles,
		diff: diffView{
			viewport:        vp,
			showLineNumbers: true,
		},
	}
	m.themePicker.normalize()
	if m.context == nil {
		m.context = context.Background()
	}
	if m.deps.findForgeCLI == nil {
		m.deps.findForgeCLI = command.FindForgeCLI
	}
	if m.deps.createPullRequest == nil {
		m.deps.createPullRequest = command.CreatePullRequest
	}
	if m.deps.mergeBranch == nil {
		m.deps.mergeBranch = command.MergeBranch
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.normalizeWorktrees()
	m.pr.reset(m.prTextInputStyles(prInputWidth), m.prTextareaStyles(prInputWidth, prBodyHeight))
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
		m.pr.submitting = false
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("PR/MR create failed: %s", msg.err))
		}
		m.mode = modeNormal
		return m, m.showSuccessToast("PR/MR created")
	case mergeBranchFinishedMsg:
		m.merge.finish()
		m.mode = modeNormal
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("merge failed: %s", msg.err))
		}
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("merged %s into %s", worktreeLabel(msg.request.Source), worktreeLabel(msg.request.Target)))}
		if m.deps.reload != nil {
			cmds = append(cmds, m.startReloadCmd(msg.request.Target.Path))
		}
		return m, tea.Batch(cmds...)
	case deleteWorktreeFinishedMsg:
		m.deletingWorktree = false
		m.mode = modeNormal
		if msg.err != nil {
			return m, m.showErrorToast(fmt.Sprintf("delete failed: %s", msg.err))
		}
		m.removeWorktree(msg.worktree.Path)
		cmds := []tea.Cmd{m.showSuccessToast(fmt.Sprintf("deleted %s", worktreeLabel(msg.worktree)))}
		if m.deps.reload != nil {
			cmds = append(cmds, m.startReloadCmd(m.selectedWorktreeValue().Path))
		}
		return m, tea.Batch(cmds...)
	case diffLoadedMsg:
		if msg.revision != m.revision {
			return m, nil
		}
		if msg.path != "" {
			key := msg.worktree + "\x00" + msg.path
			m.diffs[key] = msg.diff
			if selected := m.Selected(); selected.Path == msg.path && m.selectedWorktreeValue().Path == msg.worktree {
				m.refreshDiff()
				m.diff.viewport.SetYOffset(msg.diffYOffset)
			}
		}
		return m, nil
	case compareDiffLoadedMsg:
		if msg.generation != m.overlap.compareGeneration || m.mode != modeOverlapCompare {
			return m, nil
		}
		if m.overlap.compareTarget.Worktree.Path != msg.worktree || m.overlap.compareTarget.Change.Path != msg.path {
			return m, nil
		}
		m.overlap.compareDiff = msg.diff
		m.overlap.compareLoading = false
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.viewport.SoftWrap)
		return m, nil
	case autoRefreshMsg:
		m.revision++
		if m.deps.reload == nil {
			return m, m.autoRefreshCmd()
		}
		if m.refreshInFlight {
			return m, m.autoRefreshCmd()
		}
		return m, tea.Batch(
			m.startReloadCmd(m.selectedWorktreeValue().Path),
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
		switch m.mode {
		case modeOverlapCompare:
			return m.handleOverlapCompareWheel(msg.Mouse())
		case modeThemePicker:
			return m.handleThemeWheel(msg.Mouse())
		}
		if m.mode.blocksDiffWheel() {
			return m, nil
		}
		return m.handleDiffWheel(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	m.diff.viewport, cmd = m.diff.viewport.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFileFilter:
		return m.handleFileFilterKey(msg)
	case modeOverlapCompare:
		return m.handleOverlapCompareKey(msg)
	case modeOverlapPicker:
		return m.handleOverlapPickerKey(msg)
	case modePRForm:
		return m.handlePRFormKey(msg)
	case modeDeleteConfirm:
		return m.handleDeleteConfirmKey(msg)
	case modeMergeConfirm:
		return m.handleMergeConfirmKey(msg)
	case modeThemePicker:
		return m.handleThemeKey(msg)
	case modeMergeTarget:
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
			m.mode = modeNormal
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
		m.mode = modeFileFilter
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
	case "o":
		return m, m.openOverlapPicker()
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
	case "l", "right":
		if m.focusedPane == paneDiff {
			m.scrollDiffHorizontal(1)
			return m, nil
		}
		if m.scrollFocusedList(1) {
			return m, nil
		}
	case "0":
		if m.scrollFocusedListToStart() {
			return m, nil
		}
	case "$":
		if m.scrollFocusedListToEnd() {
			return m, nil
		}
	case "g", "home":
		if m.focusedPane == paneDiff {
			m.diff.viewport.GotoTop()
			return m, nil
		}
		m.focusedPane = paneFiles
		m.selected = 0
		m.refreshDiff()
		return m, m.ensureSelectedDiffCmd()
	case "G", "end":
		if m.focusedPane == paneDiff {
			m.diff.viewport.GotoBottom()
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
	m.diff.viewport, cmd = m.diff.viewport.Update(msg)
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
	switch m.mode {
	case modeThemePicker:
		body = m.renderOverlay(body, m.renderThemePicker())
	case modeFileFilter:
		body = m.renderOverlay(body, m.renderFileFilter())
	case modeOverlapPicker:
		body = m.renderOverlay(body, m.renderOverlapPicker())
	case modeDeleteConfirm:
		body = m.renderOverlay(body, m.renderDeleteConfirm())
	case modePRForm:
		body = m.renderOverlay(body, m.renderPRForm())
	case modeMergeConfirm:
		body = m.renderOverlay(body, m.renderMergeConfirm())
	case modeMergeTarget:
		body = m.renderOverlay(body, m.renderMergeTargetPicker())
	}
	if m.mode == modeOverlapCompare {
		body = m.renderOverlapCompare(body)
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
	if len(m.overlapTargetsFor(m.Selected())) > 0 {
		segments = append(segments, m.footerHint(iconWarning, "o", "overlaps"))
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
	if selected.Path != "" {
		if index := slices.IndexFunc(changes, func(change gitview.FileChange) bool {
			return change.Path == selected.Path
		}); index >= 0 {
			m.selected = index
			m.refreshDiff()
			return
		}
	}
	m.selected = min(fallbackIndex, max(0, len(changes)-1))
	m.refreshDiff()
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
	state, ok := m.move(delta)
	if !ok {
		return
	}
	m.focusedPane = paneWorktrees
	m.applySelectedWorktree(state)
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
	if m.diff.viewport.SoftWrap {
		m.diff.viewport.SoftWrap = false
		return m.showToast("diff wrap off")
	}
	m.diff.viewport.SetXOffset(0)
	m.diff.viewport.SoftWrap = true
	return m.showToast("diff wrap on")
}

func (m *Model) toggleLineNumbers() tea.Cmd {
	m.diff.showLineNumbers = !m.diff.showLineNumbers
	m.diff.viewport.SetXOffset(clamp(m.diff.viewport.XOffset(), 0, m.maxDiffXOffset()))
	if m.diff.showLineNumbers {
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
	worktreePath := m.selectedWorktreeValue().Path
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
	if m.diff.viewport.SoftWrap {
		return
	}
	step := 6
	m.diff.viewport.SetXOffset(clamp(m.diff.viewport.XOffset()+delta*step, 0, m.maxDiffXOffset()))
}

func (m *Model) scrollMergeConfirmHorizontal(delta int) {
	step := 6
	contentWidth := m.mergeConfirmContentWidth(m.mergeConfirmWidth())
	m.merge.confirmScrollX = clamp(m.merge.confirmScrollX+delta*step, 0, m.maxMergeConfirmScrollX(contentWidth))
}

func (m Model) maxDiffXOffset() int {
	return max(0, m.maxDiffLineWidth()-m.diffTextWidth(m.diff.viewport.Width()))
}

func (m Model) maxDiffLineWidth() int {
	width := 0
	for _, line := range m.diff.lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func (m Model) maxCompareYOffset() int {
	_, height := m.compareColumnDimensions()
	return max(0, max(m.diffDisplayLineCount(m.selectedDiff()), m.diffDisplayLineCount(m.overlap.compareDiff))-height)
}

func (m Model) maxCompareXOffset() int {
	width, _ := m.compareColumnDimensions()
	textWidth := m.diffTextWidth(width)
	return max(0, max(m.maxDiffTextLineWidth(m.selectedDiff()), m.maxDiffTextLineWidth(m.overlap.compareDiff))-textWidth)
}

func (m Model) compareColumnDimensions() (int, int) {
	if m.width < 96 {
		return 1, 1
	}
	innerWidth := panelInnerWidth(m.width)
	columnWidth := max(1, (innerWidth-1)/2)
	return columnWidth, max(1, panelInnerHeight(m.bodyHeight()))
}

func (m Model) diffDisplayLineCount(diff string) int {
	if diff == "" {
		return 1
	}
	numbered := numberedDiffLines(strings.Split(diff, "\n"))
	if !m.diff.viewport.SoftWrap {
		return len(numbered)
	}
	width, _ := m.compareColumnDimensions()
	textWidth := m.diffTextWidth(width)
	count := 0
	for _, line := range numbered {
		count += len(wrapDisplaySegments(line.text, textWidth))
	}
	return max(1, count)
}

func (m Model) maxDiffTextLineWidth(diff string) int {
	width := 0
	for _, line := range strings.Split(diff, "\n") {
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
	return max(0, lipgloss.Width(m.renderFileListLineFull(m.Selected(), m.styles.Panel.GetBackground()))-available)
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
	m.mode = modeThemePicker
	m.themePicker.open()
}

func (m *Model) openDeleteConfirm() tea.Cmd {
	worktree := m.selectedWorktreeValue()
	if worktree.Path == "" {
		return m.showErrorToast("no worktree selected")
	}
	if worktree.Protected || gitview.IsProtectedBranch(worktree.Branch) {
		return m.showErrorToast(fmt.Sprintf("protected branch %s cannot be deleted", worktreeLabel(worktree)))
	}
	m.mode = modeDeleteConfirm
	m.focusedPane = paneWorktrees
	return nil
}

func (m *Model) openPRForm() tea.Cmd {
	cli, ok := m.deps.findForgeCLI()
	if !ok {
		return m.showErrorToast("Forge CLI missing: install gh or glab")
	}
	m.pr.forgeCLI = cli
	m.mode = modePRForm
	m.focusedPane = paneWorktrees
	m.pr.reset(m.prTextInputStyles(prInputWidth), m.prTextareaStyles(prInputWidth, prBodyHeight))
	return nil
}

func (m *Model) openMergeTargetPicker() tea.Cmd {
	source := m.selectedWorktreeValue()
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
	m.merge.openTargetPicker(source, m.newMergeTargetList(items))
	m.mode = modeMergeTarget
	m.focusedPane = paneWorktrees
	return nil
}

func (m *Model) openOverlapPicker() tea.Cmd {
	targets := m.overlapTargetsFor(m.Selected())
	if len(targets) == 0 {
		return m.showToast("no overlaps for selected file")
	}
	m.overlap.openPicker(targets)
	m.mode = modeOverlapPicker
	return nil
}

func (m Model) overlapTargetsFor(change gitview.FileChange) []overlapTarget {
	if change.Path == "" {
		return nil
	}
	selectedWorktreePath := m.selectedWorktreeValue().Path
	paths := changePathSet(change)
	targets := make([]overlapTarget, 0)
	for _, state := range m.worktrees {
		if state.Worktree.Path == selectedWorktreePath {
			continue
		}
		for _, candidate := range state.Changes {
			if changesOverlap(paths, candidate) {
				targets = append(targets, overlapTarget{Worktree: state.Worktree, Change: candidate})
			}
		}
	}
	return targets
}

func (m Model) visibleOverlapCount() int {
	total := 0
	for _, change := range m.visibleChanges() {
		total += len(m.overlapTargetsFor(change))
	}
	return total
}

func changePathSet(change gitview.FileChange) map[string]struct{} {
	paths := make(map[string]struct{}, 2)
	if change.Path != "" {
		paths[change.Path] = struct{}{}
	}
	if change.OldPath != "" {
		paths[change.OldPath] = struct{}{}
	}
	return paths
}

func changesOverlap(paths map[string]struct{}, candidate gitview.FileChange) bool {
	if _, ok := paths[candidate.Path]; ok && candidate.Path != "" {
		return true
	}
	if _, ok := paths[candidate.OldPath]; ok && candidate.OldPath != "" {
		return true
	}
	return false
}

func (m Model) handleOverlapPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.mode = modeNormal
		m.overlap.closePicker()
		return m, nil
	case "j", "down":
		m.overlap.moveCursor(1)
		return m, nil
	case "k", "up":
		m.overlap.moveCursor(-1)
		return m, nil
	case "enter":
		return m, m.openOverlapCompare()
	}
	return m, nil
}

func (m *Model) openOverlapCompare() tea.Cmd {
	target, ok := m.overlap.openCompare()
	if !ok {
		return m.showToast("no overlap selected")
	}
	m.mode = modeOverlapCompare
	return m.compareDiffCmd(target)
}

func (m Model) compareDiffCmd(target overlapTarget) tea.Cmd {
	if m.deps.loadDiff == nil {
		return func() tea.Msg {
			return compareDiffLoadedMsg{
				generation: m.overlap.compareGeneration,
				worktree:   target.Worktree.Path,
				path:       target.Change.Path,
				diff:       fmt.Sprintf("No diff loader configured for %s", target.Change.Path),
			}
		}
	}
	return func() tea.Msg {
		return compareDiffLoadedMsg{
			generation: m.overlap.compareGeneration,
			worktree:   target.Worktree.Path,
			path:       target.Change.Path,
			diff:       m.deps.loadDiff(m.context, target.Worktree.Path, target.Change),
		}
	}
}

func (m Model) handleOverlapCompareKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.mode = modeNormal
		m.overlap.closeCompare()
		return m, nil
	case "w":
		cmd := m.toggleDiffWrap()
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.viewport.SoftWrap)
		return m, cmd
	case "n":
		cmd := m.toggleLineNumbers()
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.viewport.SoftWrap)
		return m, cmd
	case "j", "down":
		m.overlap.scrollCompare(1, m.maxCompareYOffset())
	case "k", "up":
		m.overlap.scrollCompare(-1, m.maxCompareYOffset())
	case "g", "home":
		m.overlap.compareYOffset = 0
	case "G", "end":
		m.overlap.compareYOffset = m.maxCompareYOffset()
	case "h", "left":
		if !m.diff.viewport.SoftWrap {
			m.overlap.scrollCompareHorizontal(-1, m.maxCompareXOffset())
		}
	case "l", "right":
		if !m.diff.viewport.SoftWrap {
			m.overlap.scrollCompareHorizontal(1, m.maxCompareXOffset())
		}
	case "0":
		if !m.diff.viewport.SoftWrap {
			m.overlap.compareXOffset = 0
		}
	case "$":
		if !m.diff.viewport.SoftWrap {
			m.overlap.compareXOffset = m.maxCompareXOffset()
		}
	}
	m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.viewport.SoftWrap)
	return m, nil
}

func (m Model) handleOverlapCompareWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	switch mouse.Button {
	case tea.MouseWheelUp:
		m.overlap.scrollCompare(-3, m.maxCompareYOffset())
	case tea.MouseWheelDown:
		m.overlap.scrollCompare(3, m.maxCompareYOffset())
	}
	return m, nil
}

func (m Model) handleDiffWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	leftWidth, _ := m.layoutWidths()
	if mouse.X < leftWidth || mouse.Y < 0 || mouse.Y >= m.bodyHeight() {
		return m, nil
	}
	m.focusedPane = paneDiff
	var cmd tea.Cmd
	m.diff.viewport, cmd = m.diff.viewport.Update(msg)
	return m, cmd
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
		if m.pr.submitting {
			return m, nil
		}
		cmd := m.createPullRequestCmd()
		return m, cmd
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "tab", "shift+tab":
		m.pr.toggleFocus()
		return m, nil
	}

	return m, m.pr.updateInput(msg)
}

func (m *Model) createPullRequestCmd() tea.Cmd {
	req, err := m.pr.request(m.selectedWorktreeValue())
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.pr.submitting = true
	return func() tea.Msg {
		return pullRequestFinishedMsg{err: m.deps.createPullRequest(m.context, req)}
	}
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
		m.mode = modeNormal
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) deleteSelectedWorktreeCmd() tea.Cmd {
	worktree := m.selectedWorktreeValue()
	if m.deps.deleteWorktree == nil {
		return func() tea.Msg {
			return deleteWorktreeFinishedMsg{worktree: worktree, err: fmt.Errorf("delete worktree is not configured")}
		}
	}
	return func() tea.Msg {
		return deleteWorktreeFinishedMsg{worktree: worktree, err: m.deps.deleteWorktree(m.context, worktree)}
	}
}

func (m Model) handleThemeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "t":
		m.mode = modeNormal
	case "j", "down":
		m.themePicker.moveCursor(1)
	case "k", "up":
		m.themePicker.moveCursor(-1)
	case " ", "space":
		if m.themePicker.cursor == 0 {
			cmd = m.toggleTransparentBackground()
		}
	case "enter":
		if m.themePicker.cursor == 0 {
			return m, nil
		} else {
			cmd = m.applyThemeCursor()
		}
		m.mode = modeNormal
	}
	return m, cmd
}

func (m Model) handleMergeTargetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "m":
		if m.merge.merging {
			return m, nil
		}
		m.mode = modeNormal
		return m, nil
	case "enter":
		if m.merge.merging {
			return m, nil
		}
		return m, m.openMergeConfirm()
	}
	return m, m.merge.updateTargetList(msg)
}

func (m *Model) openMergeConfirm() tea.Cmd {
	request, ok := m.merge.selectedRequest(m.selectedWorktreeValue())
	if !ok {
		return m.showErrorToast("no merge target branch")
	}
	m.merge.openConfirm(request)
	m.mode = modeMergeConfirm
	return nil
}

func (m Model) handleMergeConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n", "m":
		if m.merge.merging {
			return m, nil
		}
		m.mode = modeNormal
		m.merge.cancelConfirm()
		return m, nil
	case "h", "left":
		m.scrollMergeConfirmHorizontal(-1)
		return m, nil
	case "l", "right":
		m.scrollMergeConfirmHorizontal(1)
		return m, nil
	case "enter", "y":
		if m.merge.merging {
			return m, nil
		}
		return m, m.mergeConfirmedTargetCmd()
	}
	return m, nil
}

func (m *Model) mergeConfirmedTargetCmd() tea.Cmd {
	request, ok := m.merge.start()
	if !ok {
		return m.showErrorToast("no merge target branch")
	}
	return func() tea.Msg {
		return mergeBranchFinishedMsg{request: request, err: m.deps.mergeBranch(m.context, request)}
	}
}

func (m *Model) applyThemeCursor() tea.Cmd {
	name, ok := m.themePicker.selectedName()
	if !ok {
		return nil
	}
	preset, err := theme.Preset(name)
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.themePicker.name = name
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.themePicker.transparent})
	var cmd tea.Cmd
	if m.deps.saveTheme != nil {
		if err := m.deps.saveTheme(name); err != nil {
			cmd = m.showErrorToast(fmt.Sprintf("Could not save theme: %s", err))
		}
	}
	m.refreshDiff()
	return cmd
}

func (m *Model) toggleTransparentBackground() tea.Cmd {
	m.themePicker.toggleTransparent()
	preset, err := theme.Preset(m.themePicker.name)
	if err != nil {
		return m.showErrorToast(err.Error())
	}
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.themePicker.transparent})
	m.refreshDiff()
	if m.deps.saveTransparent != nil {
		if err := m.deps.saveTransparent(m.themePicker.transparent); err != nil {
			return m.showErrorToast(fmt.Sprintf("Could not save transparency: %s", err))
		}
	}
	return nil
}

func (m Model) handleFileFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Code == '\r' || msg.Code == '\n' {
		m.mode = modeNormal
		return m, m.ensureSelectedDiffCmd()
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		selected := m.Selected()
		m.fileFilter = ""
		m.mode = modeNormal
		m.restoreSelectedFile(selected, m.selected)
		return m, m.ensureSelectedDiffCmd()
	case "enter":
		m.mode = modeNormal
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
	if m.mode == modeThemePicker {
		overlay := m.renderThemePicker()
		x, y := m.overlayPosition(overlay)
		if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) {
			return false, nil
		}
		index := mouse.Y - y - 2
		if index == 0 {
			m.themePicker.setCursor(0)
			cmd := m.toggleTransparentBackground()
			return false, cmd
		}
		offset := m.themePickerOffset()
		if index > 0 && index <= m.themePickerVisibleThemeRows() && offset+index-1 < len(m.themePicker.names) {
			m.themePicker.setCursor(offset + index)
			cmd := m.applyThemeCursor()
			m.mode = modeNormal
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
		index := m.offset(worktreeHeight) + bodyY - 1
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
		m.themePicker.moveCursor(-1)
	case tea.MouseWheelDown:
		m.themePicker.moveCursor(1)
	}
	return m, nil
}

func (m Model) reloadCmd(generation int, selectedWorktreePath string) tea.Cmd {
	reload := m.deps.reload
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
	return m.ensureSelectedDiffCmdWithYOffset(m.diff.viewport.YOffset())
}

func (m Model) ensureSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, false)
}

func (m Model) reloadSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, true)
}

func (m Model) selectedDiffCmdWithYOffset(diffYOffset int, force bool) tea.Cmd {
	if m.deps.loadDiff == nil {
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
	worktreePath := m.selectedWorktreeValue().Path
	return func() tea.Msg {
		return diffLoadedMsg{
			revision:    m.revision,
			worktree:    worktreePath,
			path:        selected.Path,
			diff:        m.deps.loadDiff(m.context, worktreePath, selected),
			diffYOffset: diffYOffset,
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) (bool, int, bool) {
	selected := m.Selected()
	selectedWorktreePath := m.selectedWorktreeValue().Path
	selectedIndex := m.selected
	diffYOffset := m.diff.viewport.YOffset()
	preserveMergeTargetPicker := m.mode == modeMergeTarget
	mergeSource := m.merge.source
	mergeTargetPath := m.merge.selectedTargetPath()
	mergeRequest := m.merge.request
	preserveMergeConfirm := m.mode == modeMergeConfirm
	preserveMergingBranch := m.merge.merging
	preserveMergeConfirmScrollX := m.merge.confirmScrollX
	if preserveMergeTargetPicker || preserveMergeConfirm {
		m.mode = modeNormal
	}
	m.merge.clearTargetPicker()
	m.merge.finish()
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
			m.mode = modeMergeConfirm
			m.merge.restoreConfirm(
				request,
				preserveMergingBranch,
				clamp(preserveMergeConfirmScrollX, 0, m.maxMergeConfirmScrollX(m.mergeConfirmContentWidth(m.mergeConfirmWidth()))),
			)
		}
	}
	if m.mode != modeMergeConfirm && preserveMergeTargetPicker {
		m.restoreMergeTargetPicker(mergeSource, mergeTargetPath)
	}
	visibleChanges := m.visibleChanges()
	if selected.Path != "" {
		if index := slices.IndexFunc(visibleChanges, func(change gitview.FileChange) bool {
			return change.Path == selected.Path
		}); index >= 0 {
			m.selected = index
		} else {
			m.selected = min(selectedIndex, max(0, len(visibleChanges)-1))
		}
	} else {
		m.selected = min(selectedIndex, max(0, len(visibleChanges)-1))
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.refreshDiff()
	preservedDiffScroll := selected.Path != "" && m.Selected().Path == selected.Path && m.selectedWorktreeValue().Path == selectedWorktreePath
	reloadSelectedDiff := snapshot.Diffs == nil && preservedDiffScroll && selectedDiffChanged(selected, m.Selected())
	if preservedDiffScroll {
		m.diff.viewport.SetYOffset(diffYOffset)
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

func selectedDiffChanged(previous, next gitview.FileChange) bool {
	if previous.Status != next.Status ||
		previous.OldPath != next.OldPath ||
		previous.Additions != next.Additions ||
		previous.Deletions != next.Deletions ||
		previous.Binary != next.Binary {
		return true
	}
	if previous.Fingerprint != "" || next.Fingerprint != "" {
		return previous.Fingerprint != next.Fingerprint
	}
	return false
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
	refreshedSource, ok := m.byPath(source.Path)
	if !ok || refreshedSource.Branch == "" || refreshedSource.Branch == "detached" || m.isDefaultBranch(refreshedSource) {
		return false
	}
	items := m.mergeTargetItems(refreshedSource)
	if len(items) == 0 {
		return false
	}
	m.merge.openTargetPicker(refreshedSource, m.newMergeTargetList(items))
	if selectedTargetPath != "" {
		if index := slices.IndexFunc(items, func(item list.Item) bool {
			target, ok := item.(mergeTargetItem)
			return ok && target.worktree.Path == selectedTargetPath
		}); index >= 0 {
			m.merge.targetList.Select(index)
		}
	}
	m.mode = modeMergeTarget
	m.focusedPane = paneWorktrees
	return true
}

func (m Model) refreshedMergeRequest(request MergeRequest) (MergeRequest, bool) {
	source, sourceOK := m.byPath(request.Source.Path)
	target, targetOK := m.byPath(request.Target.Path)
	if !sourceOK || !targetOK {
		return MergeRequest{}, false
	}
	return MergeRequest{
		Source: source,
		Target: target,
	}, true
}

func (m *Model) normalizeWorktrees() {
	state := m.normalize(m.changes, m.err)
	m.changes = state.Changes
	if state.Error != nil {
		m.err = state.Error
	}
}

func (m *Model) selectWorktree(index int) {
	state, ok := m.selectIndex(index)
	if !ok {
		return
	}
	m.applySelectedWorktree(state)
}

func (m *Model) applySelectedWorktree(state WorktreeState) {
	m.changes = state.Changes
	m.err = state.Error
	m.selected = 0
	m.refreshDiff()
}

func (m *Model) removeWorktree(path string) {
	state, ok := m.remove(path, m.changes, m.err)
	if !ok {
		return
	}
	m.changes = state.Changes
	if state.Error != nil {
		m.err = state.Error
	}
	m.refreshDiff()
}

func (m *Model) resizeViewport() {
	_, rightWidth := m.layoutWidths()
	contentHeight := m.bodyHeight()
	m.diff.viewport.Style = m.styles.Diff
	m.diff.viewport.SetWidth(max(10, panelInnerWidth(rightWidth)))
	m.diff.viewport.SetHeight(max(3, panelInnerHeight(contentHeight)))
}

func (m *Model) refreshDiff() {
	m.resizeViewport()
	changes := m.visibleChanges()
	if len(changes) == 0 {
		if len(m.changes) > 0 && m.fileFilter != "" {
			m.setDiffContentFor("empty:no-matching:"+m.fileFilter, "No matching files.")
			return
		}
		m.setDiffContentFor("empty:no-changes:"+m.selectedWorktreeValue().Path, "No changes in this worktree.")
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
		m.diff.viewport.GotoTop()
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
	if key == m.diff.contentKey && diff == m.diff.content {
		return false
	}
	lines := strings.Split(diff, "\n")
	m.diff.contentKey = key
	m.diff.content = diff
	m.diff.lines = lines
	m.diff.viewport.StyleLineFunc = func(index int) lipgloss.Style {
		if index < 0 || index >= len(lines) {
			return m.styles.Diff.Inline(true).Width(m.diff.viewport.Width())
		}
		return m.diffLineStyle(lines[index]).Inline(true).Width(m.diff.viewport.Width())
	}
	m.diff.viewport.SetContent(diff)
	return true
}

func (m Model) renderWorktrees(width, height int) string {
	focused := m.focusedPane == paneWorktrees
	lines := make([]string, 0, len(m.worktrees))
	contentWidth := panelInnerWidth(width)
	visibleRows := m.visibleRows(height)
	offset := m.offset(height)
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
		return m.renderFileListLineFull(change, background)
	}
	prefixWidth := lipgloss.Width(iconSelected + " ")
	contentWidth := max(1, rowWidth-prefixWidth)
	return renderFileLineWithinWidth(m.styles, change, m.fileFilter, background, contentWidth, m.fileOverlapBadge(change, background))
}

func (m Model) renderFileListLineFull(change gitview.FileChange, background color.Color) string {
	return renderFileLineWithBackground(m.styles, change, m.fileFilter, background, m.fileOverlapBadge(change, background))
}

func (m Model) fileOverlapBadge(change gitview.FileChange, background color.Color) string {
	count := len(m.overlapTargetsFor(change))
	if count == 0 {
		return ""
	}
	return listFillWithBackground(background, " ") +
		listStyleWithBackground(m.styles.Muted, background).Render(iconWarning) +
		listFillWithBackground(background, " ") +
		listStyleWithBackground(m.styles.Muted, background).Render(fmt.Sprintf("overlap %d", count))
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
	overlaps := m.visibleOverlapCount()
	if overlaps > 0 {
		title += fmt.Sprintf("  %d overlaps", overlaps)
	}
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
	worktree := m.selectedWorktreeValue()
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
		renderOverlayLine(lineStyle, width, "Branch: "+worktreeLabel(worktree), 0),
		renderOverlayLine(lineStyle, width, "Path: "+worktree.Path, 0),
		renderOverlayLine(lineStyle, width, "Changes: "+changedFilesText(len(m.changes)), 0),
		lineStyle.Width(width).Render(""),
		lineStyle.Width(width).Align(lipgloss.Center).Render("remove worktree and delete branch"),
		lineStyle.Width(width).Render(""),
		options,
	}
	return panel.Width(width+4).Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func changedFilesText(count int) string {
	if count == 1 {
		return "1 changed file"
	}
	return fmt.Sprintf("%d changed files", count)
}

func (m Model) confirmButton(key, label string) string {
	keyText := m.styles.DiffHunk.Bold(true).Render("[" + key + "]")
	return keyText + m.styles.Diff.Render(label)
}

func (m Model) renderPRForm() string {
	width := min(max(48, m.width-8), 78)
	titleInput := m.pr.title
	bodyInput := m.pr.body
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
		Render(iconPR + " Forge CLI: " + m.pr.forgeCLI)
	title := m.renderPanel(width, 3, m.pr.focus == prTitle, "PR title", titleInput.View())
	bodyTitle := "PR description    <tab> focus    <c-o> create"
	body := m.renderPanel(width, bodyPanelHeight, m.pr.focus == prBody, bodyTitle, bodyInput.View())
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
	end := min(len(m.themePicker.names), offset+m.themePickerVisibleThemeRows())
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
	if index == m.themePicker.cursor {
		prefix = iconSelected + " "
	}
	line := prefix + m.themePickerRowLabel(index)
	if index == m.themePicker.cursor {
		return m.styles.FileSelected.Width(width).Render(line)
	}
	return m.styles.Diff.Width(width).Render(line)
}

func (m Model) themePickerRowLabel(index int) string {
	if index == 0 {
		state := iconToggleOff
		if m.themePicker.transparent {
			state = iconToggleOn
		}
		return "Transparent background  " + state
	}
	themeIndex := index - 1
	if themeIndex < 0 || themeIndex >= len(m.themePicker.names) {
		return ""
	}
	return m.themePicker.names[themeIndex]
}

func (m Model) renderThemePickerFooter(width int) string {
	label := m.themePickerPositionLabel()
	if m.themePicker.cursor == 0 {
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
	total := len(m.themePicker.names)
	if total == 0 {
		return "0/0"
	}
	current := clamp(m.themePicker.cursor, 0, total)
	return fmt.Sprintf("%d/%d", current, total)
}

func (m Model) renderMergeTargetPicker() string {
	targets := m.merge.targetList
	width := min(max(40, m.width-12), 70)
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	height := min(max(6, len(targets.Items())*2+2), max(6, m.bodyHeight()-5))
	targets.SetSize(contentWidth, height)
	source := renderOverlayLine(m.styles.Muted, contentWidth, "From: "+worktreeLabel(m.merge.source)+"  >  Target: "+selectedMergeTargetLabel(targets), 0)
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
	scrollX := clamp(m.merge.confirmScrollX, 0, m.maxMergeConfirmScrollX(contentWidth))
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

func (m Model) renderOverlapPicker() string {
	width := min(max(44, m.width-12), 78)
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	title := m.styles.Title.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(iconWarning + " Overlaps for " + m.Selected().Path)
	lines := []string{title}
	visibleRows := min(len(m.overlap.targets), max(1, m.bodyHeight()-8))
	offset := m.overlapPickerOffset(visibleRows)
	end := min(len(m.overlap.targets), offset+visibleRows)
	for i := offset; i < end; i++ {
		lines = append(lines, m.renderOverlapPickerRow(i, contentWidth))
	}
	if len(m.overlap.targets) == 0 {
		lines = append(lines, m.styles.Muted.Background(panel.GetBackground()).Width(contentWidth).Render("No overlaps"))
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m Model) overlapPickerOffset(visibleRows int) int {
	if visibleRows <= 0 || len(m.overlap.targets) <= visibleRows || m.overlap.cursor < visibleRows {
		return 0
	}
	return min(m.overlap.cursor-visibleRows+1, len(m.overlap.targets)-visibleRows)
}

func (m Model) renderOverlapPickerRow(index, width int) string {
	target := m.overlap.targets[index]
	style := m.styles.Diff.Background(m.overlayPanelStyle().GetBackground())
	prefix := "  "
	if index == m.overlap.cursor {
		style = m.styles.FileSelected
		prefix = iconSelected + " "
	}
	label := prefix + worktreeLabel(target.Worktree)
	if target.Worktree.Path != "" {
		label += "  " + target.Worktree.Path
	}
	return renderOverlayLine(style, width, label, 0)
}

func (m Model) renderOverlapCompare(background string) string {
	width := m.width
	height := m.bodyHeight()
	if width < 96 {
		return m.renderOverlay(background, m.renderOverlapCompareNarrowMessage())
	}
	return m.renderOverlapComparePanel(width, height)
}

func (m Model) renderOverlapCompareNarrowMessage() string {
	width := min(max(42, m.width-8), 64)
	panel := m.overlayPanelStyle().Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	lines := []string{
		m.styles.Title.Background(panel.GetBackground()).Width(contentWidth).Render(iconWarning + " Compare"),
		m.styles.Diff.Background(panel.GetBackground()).Width(contentWidth).Render("Widen the terminal for side-by-side compare."),
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (m Model) renderOverlapComparePanel(width, height int) string {
	width = max(4, width)
	height = max(4, height)
	title := fmt.Sprintf("[3]-Compare %s ↔ %s  %s", worktreeLabel(m.selectedWorktreeValue()), worktreeLabel(m.overlap.compareTarget.Worktree), m.Selected().Path)
	innerWidth := panelInnerWidth(width)
	innerHeight := panelInnerHeight(height)
	dividerWidth := 1
	columnWidth := max(1, (innerWidth-dividerWidth)/2)
	contentHeight := innerHeight
	leftDiff := m.selectedDiff()
	if leftDiff == "" {
		leftDiff = fmt.Sprintf("No diff loaded for %s", m.Selected().Path)
	}
	rightDiff := m.overlap.compareDiff
	if m.overlap.compareLoading {
		rightDiff = "Loading overlap diff..."
	} else if rightDiff == "" {
		rightDiff = fmt.Sprintf("No diff loaded for %s", m.overlap.compareTarget.Change.Path)
	}
	left := strings.Split(m.renderDiffTextAt(leftDiff, columnWidth, contentHeight, m.overlap.compareYOffset, m.overlap.compareXOffset), "\n")
	right := strings.Split(m.renderDiffTextAt(rightDiff, columnWidth, contentHeight, m.overlap.compareYOffset, m.overlap.compareXOffset), "\n")
	divider := m.styles.Muted.Background(m.styles.Diff.GetBackground()).Render("│")
	lines := make([]string, 0, contentHeight)
	for i := range contentHeight {
		leftLine, rightLine := "", ""
		if i < len(left) {
			leftLine = left[i]
		}
		if i < len(right) {
			rightLine = right[i]
		}
		lines = append(lines, leftLine+divider+rightLine)
	}
	return m.renderPanelWithFillStyles(width, height, true, title, strings.Join(lines, "\n"), m.styles.Diff, m.styles.Diff)
}

func (m Model) renderDiffTextAt(diff string, width, height, yOffset, xOffset int) string {
	width = max(1, width)
	height = max(1, height)
	textWidth := m.diffTextWidth(width)
	numbered := numberedDiffLines(strings.Split(diff, "\n"))
	if m.diff.viewport.SoftWrap {
		return m.renderWrappedDiffLines(numbered, width, textWidth, height, yOffset)
	}
	return m.renderUnwrappedDiffLines(numbered, width, textWidth, height, yOffset, xOffset)
}

func (m Model) renderWrappedDiffLines(numbered []numberedDiffLine, width, textWidth, height, yOffset int) string {
	lines := make([]string, 0, height)
	seen := 0
	for _, line := range numbered {
		style := m.diffLineStyle(line.text)
		highlight := shouldHighlightDiffSyntaxLine(line)
		segments := wrapDisplaySegments(line.text, textWidth)
		for segmentIndex, segment := range segments {
			if seen >= yOffset {
				lines = append(lines, m.renderDiffSegment(style, m.lineNumberGutter(line, segmentIndex > 0), segment, width, textWidth, highlight))
				if len(lines) == height {
					return strings.Join(lines, "\n")
				}
			}
			seen++
		}
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (m Model) renderUnwrappedDiffLines(numbered []numberedDiffLine, width, textWidth, height, yOffset, xOffset int) string {
	lines := make([]string, 0, height)
	for i := yOffset; i < len(numbered) && len(lines) < height; i++ {
		line := numbered[i]
		segment := ansi.Cut(line.text, xOffset, xOffset+textWidth)
		lines = append(lines, m.renderDiffSegment(m.diffLineStyle(line.text), m.lineNumberGutter(line, false), segment, width, textWidth, shouldHighlightDiffSyntaxLine(line)))
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (m Model) mergeConfirmTextLines() []string {
	request := m.merge.request
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
	themeCursor := m.themePicker.selectedThemeIndex()
	if visibleRows <= 0 || len(m.themePicker.names) <= visibleRows || themeCursor < visibleRows {
		return 0
	}
	return min(themeCursor-visibleRows+1, len(m.themePicker.names)-visibleRows)
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
	return m.themePicker.totalRows()
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
	return m.selectedWorktreeValue().Path + "\x00" + change.Path
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

func renderFileLineWithBackground(styles theme.Styles, change gitview.FileChange, filter string, background color.Color, suffixParts ...string) string {
	suffix := ""
	if len(suffixParts) > 0 {
		suffix = suffixParts[0]
	}
	status := statusIcon(change.Status)
	return listStyleWithBackground(styles.Muted, background).Render(status) +
		listFillWithBackground(background, " ") +
		renderFilteredPathWithBackground(styles, change.Path, filter, background) +
		fileLineCounts(styles, change, background) +
		suffix
}

func renderFileLineWithinWidth(styles theme.Styles, change gitview.FileChange, filter string, background color.Color, width int, suffix string) string {
	status := listStyleWithBackground(styles.Muted, background).Render(statusIcon(change.Status))
	space := listFillWithBackground(background, " ")
	counts := fileLineCounts(styles, change, background)
	pathWidth := max(0, width-lipgloss.Width(status)-lipgloss.Width(space)-lipgloss.Width(counts)-lipgloss.Width(suffix))
	path := middleEllipsizePath(change.Path, pathWidth)
	return status + space + renderFilteredPathWithBackground(styles, path, filter, background) + counts + suffix
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

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
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

type compareDiffLoadedMsg struct {
	generation int
	worktree   string
	path       string
	diff       string
}
