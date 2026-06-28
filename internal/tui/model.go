package tui

import (
	"context"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/command"
	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui/components"
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

type focusedPane int

const (
	paneWorktrees focusedPane = iota
	paneFiles
	paneDiff
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
	mode              uiMode
	focusedPane       focusedPane
	diff              components.Diff
	panel             components.Panel
	footer            components.Footer
	confirm           components.Confirm
	toast             components.Toast
	pr                components.PR
	merge             merge
	overlap           overlap
	themePicker       components.ThemePicker
	worktreeList
	loadDiff          func(context.Context, string, gitview.FileChange) string
	deleteWorktree    func(context.Context, gitview.Worktree) error
	reload            func(context.Context, string) Snapshot
	saveTheme         func(string) error
	saveTransparent   func(bool) error
	findForgeCLI      func() (string, bool)
	createPullRequest func(context.Context, PullRequestRequest) error
	mergeBranch       func(context.Context, MergeRequest) error
}

func NewModel(cfg Config) Model {
	m := Model{
		styles:            cfg.Theme,
		panel:             components.NewPanel(cfg.Theme),
		footer:            components.NewFooter(cfg.Theme),
		context:           cfg.Context,
		themePicker:       components.NewThemePicker(cfg.ThemeName, cfg.Transparent, cfg.ThemeNames),
		worktreeList:      worktreeList{worktrees: cfg.Worktrees, selectedWorktree: cfg.SelectedWorktree},
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
		width:             initialDimension(cfg.Width, 100),
		height:            initialDimension(cfg.Height, 30),
		focusedPane:       paneFiles,
		diff:              components.NewDiff(cfg.Theme),
		pr:                components.NewPR(cfg.Theme),
	}
	m.themePicker.Normalize()
	if m.context == nil {
		m.context = context.Background()
	}
	if m.findForgeCLI == nil {
		m.findForgeCLI = command.FindForgeCLI
	}
	if m.createPullRequest == nil {
		m.createPullRequest = command.CreatePullRequest
	}
	if m.mergeBranch == nil {
		m.mergeBranch = command.MergeBranch
	}
	if m.diffs == nil {
		m.diffs = map[string]string{}
	}
	m.normalizeWorktrees()
	m.confirm.SetStyles(m.styles, m.overlayPanelStyle(), m.confirmWidth())
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
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.confirm.IsSubmitting() {
			return m, nil
		}
		return m, m.confirm.Update(msg)
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
	case components.ToastExpiredMsg:
		m.toast.ClearExpired(msg.ID)
		return m, nil
	case editorFinishedMsg:
		if msg.err != nil {
			return m, m.toast.Error(fmt.Sprintf("editor failed: %s", msg.err))
		}
		return m, nil
	case pullRequestFinishedMsg:
		m.pr.FinishSubmit()
		if msg.err != nil {
			return m, m.toast.Error(fmt.Sprintf("PR/MR create failed: %s", msg.err))
		}
		m.mode = modeNormal
		return m, m.toast.Success("PR/MR created")
	case mergeBranchFinishedMsg:
		m.merge.finish()
		m.confirm.Close()
		m.mode = modeNormal
		if msg.err != nil {
			return m, m.toast.Error(fmt.Sprintf("merge failed: %s", msg.err))
		}
		cmds := []tea.Cmd{m.toast.Success(fmt.Sprintf("merged %s into %s", worktreeLabel(msg.request.Source), worktreeLabel(msg.request.Target)))}
		if m.reload != nil {
			cmds = append(cmds, m.startReloadCmd(msg.request.Target.Path))
		}
		return m, tea.Batch(cmds...)
	case deleteWorktreeFinishedMsg:
		m.confirm.Close()
		m.mode = modeNormal
		if msg.err != nil {
			return m, m.toast.Error(fmt.Sprintf("delete failed: %s", msg.err))
		}
		m.removeWorktree(msg.worktree.Path)
		cmds := []tea.Cmd{m.toast.Success(fmt.Sprintf("deleted %s", worktreeLabel(msg.worktree)))}
		if m.reload != nil {
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
			if selected := m.selectedFileValue(); selected.Path == msg.path && m.selectedWorktreeValue().Path == msg.worktree {
				m.refreshDiff()
				m.diff.SetYOffset(msg.diffYOffset)
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
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.SoftWrap())
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
			m.startReloadCmd(m.selectedWorktreeValue().Path),
			m.autoRefreshCmd(),
		)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		m.confirm.SetStyles(m.styles, m.overlayPanelStyle(), m.confirmWidth())
	case tea.MouseClickMsg:
		changed, mouseCmd := m.handleMouse(msg.Mouse())
		if changed {
			if mouseCmd != nil {
				return m, mouseCmd
			}
			return m, m.ensureSelectedDiffCmd()
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

	return m, m.diff.Update(msg)
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
			selected := m.selectedFileValue()
			m.fileFilter = ""
			m.mode = modeNormal
			return m, m.restoreSelectedFile(selected, m.selected)
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
		return m, m.moveWorktree(1)
	case "shift+tab":
		return m, m.moveWorktree(-1)
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
			return m, m.moveWorktree(1)
		}
		if m.focusedPane == paneDiff {
			break
		}
		return m, m.moveSelection(1)
	case "k", "up":
		if m.focusedPane == paneWorktrees {
			return m, m.moveWorktree(-1)
		}
		if m.focusedPane == paneDiff {
			break
		}
		return m, m.moveSelection(-1)
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
			m.diff.GotoTop()
			return m, nil
		}
		m.focusedPane = paneFiles
		previous := m.selectedFileValue()
		previousWorktree := m.selectedWorktreeValue().Path
		m.selected = 0
		m.refreshDiff()
		return m, m.selectedDiffCmdAfterSelection(previous, previousWorktree)
	case "G", "end":
		if m.focusedPane == paneDiff {
			m.diff.GotoBottom()
			return m, nil
		}
		changes := m.visibleChanges()
		if len(changes) > 0 {
			m.focusedPane = paneFiles
			previous := m.selectedFileValue()
			previousWorktree := m.selectedWorktreeValue().Path
			m.selected = len(changes) - 1
			m.refreshDiff()
			return m, m.selectedDiffCmdAfterSelection(previous, previousWorktree)
		}
	}
	return m, m.diff.Update(msg)
}

func (m Model) View() tea.View {
	leftWidth, rightWidth := m.layoutWidths()
	contentHeight := m.bodyHeight()

	worktreeHeight := m.worktreePaneHeight(contentHeight)
	worktrees := m.render(m.styles, m.panel, m.focusedPane == paneWorktrees, leftWidth, worktreeHeight)
	files := m.fileListComponent().Render(m.styles, m.panel, m.focusedPane == paneFiles, leftWidth, max(4, contentHeight-lipgloss.Height(worktrees)), m.fileBadge(), m.visibleOverlapCount())
	sidebar := lipgloss.JoinVertical(lipgloss.Left, worktrees, files)
	diff := m.renderDiff(rightWidth, contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, diff)

	footer := m.footer.Render(m.width, m.footerText(), m.err)
	switch m.mode {
	case modeThemePicker:
		body = components.RenderOverlay(body, m.themePicker.Render(m.width, m.bodyHeight(), m.styles, m.overlayPanelStyle()), m.width, m.bodyHeight())
	case modeFileFilter:
		body = components.RenderOverlay(body, m.fileListComponent().RenderFilter(m.styles, m.overlayPanelStyle(), m.width), m.width, m.bodyHeight())
	case modeOverlapPicker:
		body = components.RenderOverlay(body, m.overlap.picker(m.selectedFileValue().Path).Render(m.width, m.bodyHeight(), m.styles, m.overlayPanelStyle()), m.width, m.bodyHeight())
	case modeDeleteConfirm:
		body = components.RenderOverlay(body, m.confirm.Render(), m.width, m.bodyHeight())
	case modePRForm:
		body = components.RenderOverlay(body, m.pr.Render(m.width, m.bodyHeight(), m.styles, m.panel, m.overlayPanelStyle()), m.width, m.bodyHeight())
	case modeMergeConfirm:
		body = components.RenderOverlay(body, m.confirm.Render(), m.width, m.bodyHeight())
	case modeMergeTarget:
		body = components.RenderOverlay(body, m.merge.picker.Render(m.width, m.bodyHeight(), m.styles, m.overlayPanelStyle()), m.width, m.bodyHeight())
	}
	if m.mode == modeOverlapCompare {
		body = m.overlap.compare(m.selectedWorktreeValue(), m.selectedFileValue(), m.selectedDiff()).Render(body, m.width, m.bodyHeight(), m.styles, m.panel, m.overlayPanelStyle(), m.diff.RenderTextAt)
	}
	body = m.toast.Render(body, m.styles, m.overlayPanelStyle(), m.width)

	view := tea.NewView(m.styles.App.Width(m.width).Height(m.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, body, footer),
	))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m Model) footerText() string {
	hints := []components.FooterHint{
		{Icon: components.IconKey, Key: "1/2/3", Label: "panels"},
		{Icon: components.IconWorktree, Key: "tab", Label: "worktree"},
	}
	switch m.focusedPane {
	case paneWorktrees:
		hints = append(hints,
			components.FooterHint{Icon: components.IconFile, Key: "hjkl", Label: "move"},
			components.FooterHint{Icon: components.IconDeleted, Key: "d", Label: "delete"},
			components.FooterHint{Icon: components.IconPR, Key: "p", Label: "PR"},
			components.FooterHint{Icon: components.IconMerge, Key: "m", Label: "merge"},
		)
	case paneDiff:
		hints = append(hints,
			components.FooterHint{Icon: components.IconFile, Key: "hjkl", Label: "scroll"},
			components.FooterHint{Icon: components.IconEdit, Key: "e", Label: "edit"},
			components.FooterHint{Icon: components.IconWrap, Key: "w", Label: "wrap"},
			components.FooterHint{Icon: components.IconNumbers, Key: "n", Label: "nums"},
		)
	default:
		hints = append(hints,
			components.FooterHint{Icon: components.IconFile, Key: "hjkl", Label: "move"},
			components.FooterHint{Icon: components.IconFile, Key: "0/$", Label: "edge"},
			components.FooterHint{Icon: components.IconFile, Key: "/", Label: "filter"},
			components.FooterHint{Icon: components.IconEdit, Key: "e", Label: "edit"},
		)
	}
	if len(m.overlapTargetsFor(m.selectedFileValue())) > 0 {
		hints = append(hints, components.FooterHint{Icon: components.IconWarning, Key: "o", Label: "overlaps"})
	}
	hints = append(hints,
		components.FooterHint{Icon: components.IconTheme, Key: "t", Label: "themes"},
		components.FooterHint{Icon: components.IconQuit, Key: "q", Label: "quit"},
	)
	return m.footer.Text(hints)
}

func (m Model) autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m Model) selectedFileValue() gitview.FileChange {
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

func (m *Model) restoreSelectedFile(selected gitview.FileChange, fallbackIndex int) tea.Cmd {
	previousWorktree := m.selectedWorktreeValue().Path
	list := m.fileListComponent()
	list.RestoreSelected(selected.Path, fallbackIndex)
	m.selected = list.Selected
	m.refreshDiff()
	return m.selectedDiffCmdAfterSelection(selected, previousWorktree)
}

func (m *Model) moveSelection(delta int) tea.Cmd {
	previous := m.selectedFileValue()
	previousWorktree := m.selectedWorktreeValue().Path
	list := m.fileListComponent()
	if !list.MoveSelection(delta) {
		return m.ensureSelectedDiffCmd()
	}
	m.selected = list.Selected
	m.focusedPane = paneFiles
	m.refreshDiff()
	return m.selectedDiffCmdAfterSelection(previous, previousWorktree)
}

func (m Model) fileListComponent() components.FileList {
	return components.FileList{
		Items:    fileItems(m.changes),
		Selected: m.selected,
		ScrollX:  m.fileScrollX,
		Filter:   m.fileFilter,
	}
}

func (m Model) fileBadge() components.FileBadgeFunc {
	return func(item components.FileItem, background color.Color) string {
		for _, change := range m.changes {
			if change.Path == item.ID {
				return m.fileOverlapBadge(change, background)
			}
		}
		return ""
	}
}

func (m *Model) moveWorktree(delta int) tea.Cmd {
	previous := m.selectedFileValue()
	previousWorktree := m.selectedWorktreeValue().Path
	state, ok := m.move(delta)
	if !ok {
		return m.ensureSelectedDiffCmd()
	}
	m.focusedPane = paneWorktrees
	m.applySelectedWorktree(state)
	return m.selectedDiffCmdAfterSelection(previous, previousWorktree)
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
	if !m.diff.ToggleWrap() {
		return m.toast.Info("diff wrap off")
	}
	return m.toast.Info("diff wrap on")
}

func (m *Model) toggleLineNumbers() tea.Cmd {
	if m.diff.ToggleLineNumbers() {
		return m.toast.Info("line numbers on")
	}
	return m.toast.Info("line numbers off")
}

func (m *Model) openSelectedFileInEditor() tea.Cmd {
	selected := m.selectedFileValue()
	if selected.Path == "" {
		return m.toast.Info("no file selected")
	}
	line := editorTargetLine(m.selectedDiff())
	cmd := command.OpenEditorCommand("", m.selectedWorktreeValue().Path, selected.Path, line)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m *Model) scrollDiffHorizontal(delta int) {
	if m.diff.SoftWrap() {
		return
	}
	step := 6
	m.diff.SetXOffset(clamp(m.diff.XOffset()+delta*step, 0, m.maxDiffXOffset()))
}

func (m Model) maxDiffXOffset() int {
	return m.diff.MaxXOffset()
}

func (m Model) maxCompareYOffset() int {
	_, height := m.compareColumnDimensions()
	return max(0, max(m.diffDisplayLineCount(m.selectedDiff()), m.diffDisplayLineCount(m.overlap.compareDiff))-height)
}

func (m Model) maxCompareXOffset() int {
	width, _ := m.compareColumnDimensions()
	textWidth := m.diff.TextWidth(width)
	return max(0, max(m.diff.MaxTextLineWidth(m.selectedDiff()), m.diff.MaxTextLineWidth(m.overlap.compareDiff))-textWidth)
}

func (m Model) compareColumnDimensions() (int, int) {
	if m.width < 96 {
		return 1, 1
	}
	innerWidth := components.FrameInnerWidth(m.width)
	columnWidth := max(1, (innerWidth-1)/2)
	return columnWidth, max(1, components.FrameInnerHeight(m.bodyHeight()))
}

func (m Model) diffDisplayLineCount(diff string) int {
	width, _ := m.compareColumnDimensions()
	return m.diff.DisplayLineCount(diff, width)
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
	available := m.listContentWidthForPane(paneWorktrees)
	return m.maxScrollX(m.styles, available)
}

func (m Model) maxFileScrollX() int {
	available := m.listContentWidthForPane(paneFiles)
	return m.fileListComponent().MaxScrollX(m.styles, available, m.fileBadge())
}

func (m Model) listContentWidthForPane(_ focusedPane) int {
	leftWidth, _ := m.layoutWidths()
	width := components.FrameInnerWidth(leftWidth)
	return max(1, width-lipgloss.Width(components.IconSelected+" "))
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
	m.themePicker.Open()
}

func (m *Model) openPRForm() tea.Cmd {
	cli, ok := m.findForgeCLI()
	if !ok {
		return m.toast.Error("Forge CLI missing: install gh or glab")
	}
	m.pr.SetForgeCLI(cli)
	m.mode = modePRForm
	m.focusedPane = paneWorktrees
	m.pr.Reset(m.styles)
	return nil
}

func (m *Model) openDeleteConfirm() tea.Cmd {
	worktree := m.selectedWorktreeValue()
	if worktree.Path == "" {
		return m.toast.Error("no worktree selected")
	}
	if worktree.Protected || gitview.IsProtectedBranch(worktree.Branch) {
		return m.toast.Error(fmt.Sprintf("protected branch %s cannot be deleted", worktreeLabel(worktree)))
	}
	m.confirm.Open("DELETE "+worktreeLabel(worktree)+"/", strings.Join([]string{
		"Branch: " + worktreeLabel(worktree),
		"Path: " + worktree.Path,
		"Changes: " + changedFilesText(len(m.changes)),
		"",
		"remove worktree and delete branch",
	}, "\n"))
	m.mode = modeDeleteConfirm
	m.focusedPane = paneWorktrees
	return nil
}

func (m Model) handleDeleteConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.confirm.Choice(msg.String()) {
	case components.ConfirmYes:
		if !m.confirm.Submit() {
			return m, nil
		}
		return m, tea.Batch(m.deleteSelectedWorktreeCmd(), m.confirm.Tick)
	case components.ConfirmNo:
		if m.confirm.IsSubmitting() {
			return m, nil
		}
		m.confirm.Close()
		m.mode = modeNormal
		return m, nil
	case components.ConfirmQuit:
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) deleteSelectedWorktreeCmd() tea.Cmd {
	worktree := m.selectedWorktreeValue()
	if m.deleteWorktree == nil {
		return func() tea.Msg {
			return deleteWorktreeFinishedMsg{worktree: worktree, err: fmt.Errorf("delete worktree is not configured")}
		}
	}
	return func() tea.Msg {
		return deleteWorktreeFinishedMsg{worktree: worktree, err: m.deleteWorktree(m.context, worktree)}
	}
}

func (m *Model) openMergeTargetPicker() tea.Cmd {
	source := m.selectedWorktreeValue()
	if source.Path == "" {
		return m.toast.Error("no worktree selected")
	}
	if source.Branch == "" || source.Branch == "detached" {
		return m.toast.Error("selected worktree has no branch")
	}
	if m.isDefaultBranch(source) {
		return m.toast.Info("default branch is not a merge source")
	}
	targets := mergeTargets(source, m.worktrees, m.defaultBranchName())
	if len(targets) == 0 {
		return m.toast.Error("no merge target branch")
	}
	m.merge.openTargetPicker(source, targets, "", m.styles, m.width, m.bodyHeight())
	m.mode = modeMergeTarget
	m.focusedPane = paneWorktrees
	return nil
}

func (m *Model) openOverlapPicker() tea.Cmd {
	targets := m.overlapTargetsFor(m.selectedFileValue())
	if len(targets) == 0 {
		return m.toast.Info("no overlaps for selected file")
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
		return m.toast.Info("no overlap selected")
	}
	m.mode = modeOverlapCompare
	return m.compareDiffCmd(target)
}

func (m Model) compareDiffCmd(target overlapTarget) tea.Cmd {
	if m.loadDiff == nil {
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
			diff:       m.loadDiff(m.context, target.Worktree.Path, target.Change),
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
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.SoftWrap())
		return m, cmd
	case "n":
		cmd := m.toggleLineNumbers()
		m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.SoftWrap())
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
		if !m.diff.SoftWrap() {
			m.overlap.scrollCompareHorizontal(-1, m.maxCompareXOffset())
		}
	case "l", "right":
		if !m.diff.SoftWrap() {
			m.overlap.scrollCompareHorizontal(1, m.maxCompareXOffset())
		}
	case "0":
		if !m.diff.SoftWrap() {
			m.overlap.compareXOffset = 0
		}
	case "$":
		if !m.diff.SoftWrap() {
			m.overlap.compareXOffset = m.maxCompareXOffset()
		}
	}
	m.overlap.clampCompareOffsets(m.maxCompareYOffset(), m.maxCompareXOffset(), m.diff.SoftWrap())
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
	return m, m.diff.Update(msg)
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

func (m Model) handlePRFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+o" || (msg.Code == 'o' && msg.Mod == tea.ModCtrl) {
		if m.pr.IsSubmitting() {
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
		m.pr.ToggleFocus()
		return m, nil
	}

	return m, m.pr.UpdateInput(msg)
}

func (m *Model) createPullRequestCmd() tea.Cmd {
	req, err := m.pullRequestRequest()
	if err != nil {
		return m.toast.Error(err.Error())
	}
	if !m.pr.Submit() {
		return nil
	}
	return func() tea.Msg {
		return pullRequestFinishedMsg{err: m.createPullRequest(m.context, req)}
	}
}

func (m Model) pullRequestRequest() (PullRequestRequest, error) {
	title := m.pr.Title()
	if title == "" {
		return PullRequestRequest{}, fmt.Errorf("PR title is required")
	}
	worktree := m.selectedWorktreeValue()
	if worktree.Path == "" {
		return PullRequestRequest{}, fmt.Errorf("no worktree selected")
	}
	if worktree.Branch == "" || worktree.Branch == "detached" {
		return PullRequestRequest{}, fmt.Errorf("selected worktree has no branch")
	}
	return PullRequestRequest{
		CLI:         m.pr.ForgeCLI(),
		WorktreeDir: worktree.Path,
		Branch:      worktree.Branch,
		Title:       title,
		Body:        m.pr.Body(),
	}, nil
}

func (m Model) handleThemeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	return m.handleThemePickerAction(m.themePicker.HandleKey(msg.String()))
}

func (m Model) handleThemePickerAction(action components.ThemePickerAction) (tea.Model, tea.Cmd) {
	switch action {
	case components.ThemePickerQuit:
		return m, tea.Quit
	case components.ThemePickerCancel:
		m.mode = modeNormal
		return m, nil
	case components.ThemePickerToggleTransparent:
		return m, m.toggleTransparentBackground()
	case components.ThemePickerApply:
		cmd := m.applyThemeCursor()
		m.mode = modeNormal
		return m, cmd
	default:
		return m, nil
	}
}

func (m Model) handleMergeTargetKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	action, cmd := m.merge.picker.HandleKey(msg)
	switch action {
	case components.MergePickerQuit:
		return m, tea.Quit
	case components.MergePickerCancel:
		m.mode = modeNormal
		return m, nil
	case components.MergePickerConfirm:
		return m, m.openMergeConfirm()
	default:
		return m, cmd
	}
}

func (m *Model) openMergeConfirm() tea.Cmd {
	request, ok := m.selectedMergeRequest()
	if !ok {
		return m.toast.Error("no merge target branch")
	}
	m.merge.openConfirm(request)
	title, message := mergeConfirmText(request)
	m.confirm.Open(title, message)
	m.mode = modeMergeConfirm
	return nil
}

func (m Model) selectedMergeRequest() (MergeRequest, bool) {
	source := m.merge.source
	if source.Path == "" {
		source = m.selectedWorktreeValue()
	}
	return selectedMergeRequest(source, m.worktrees, m.merge.selectedTargetPath())
}

func (m Model) handleMergeConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.confirm.Choice(msg.String()) {
	case components.ConfirmYes:
		return m, m.mergeConfirmedTargetCmd()
	case components.ConfirmNo:
		if m.confirm.IsSubmitting() {
			return m, nil
		}
		m.confirm.Close()
		m.merge.cancelConfirm()
		m.mode = modeNormal
		return m, nil
	case components.ConfirmQuit:
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m *Model) mergeConfirmedTargetCmd() tea.Cmd {
	request := m.merge.request
	if request.Source.Path == "" || request.Target.Path == "" {
		return m.toast.Error("no merge target branch")
	}
	if !m.confirm.Submit() {
		return nil
	}
	return tea.Batch(func() tea.Msg {
		return mergeBranchFinishedMsg{request: request, err: m.mergeBranch(m.context, request)}
	}, m.confirm.Tick)
}

func (m *Model) applyThemeCursor() tea.Cmd {
	name, ok := m.themePicker.SelectedName()
	if !ok {
		return nil
	}
	preset, err := theme.Preset(name)
	if err != nil {
		return m.toast.Error(err.Error())
	}
	m.themePicker.Name = name
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.themePicker.Transparent})
	m.panel.SetStyles(m.styles)
	m.footer.SetStyles(m.styles)
	m.diff.SetStyles(m.styles)
	m.pr.SetStyles(m.styles)
	m.confirm.SetStyles(m.styles, m.overlayPanelStyle(), m.confirmWidth())
	var cmd tea.Cmd
	if m.saveTheme != nil {
		if err := m.saveTheme(name); err != nil {
			cmd = m.toast.Error(fmt.Sprintf("Could not save theme: %s", err))
		}
	}
	m.refreshDiff()
	return cmd
}

func (m *Model) toggleTransparentBackground() tea.Cmd {
	m.themePicker.ToggleTransparent()
	preset, err := theme.Preset(m.themePicker.Name)
	if err != nil {
		return m.toast.Error(err.Error())
	}
	m.styles = theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: m.themePicker.Transparent})
	m.panel.SetStyles(m.styles)
	m.footer.SetStyles(m.styles)
	m.diff.SetStyles(m.styles)
	m.pr.SetStyles(m.styles)
	m.confirm.SetStyles(m.styles, m.overlayPanelStyle(), m.confirmWidth())
	m.refreshDiff()
	if m.saveTransparent != nil {
		if err := m.saveTransparent(m.themePicker.Transparent); err != nil {
			return m.toast.Error(fmt.Sprintf("Could not save transparency: %s", err))
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
		selected := m.selectedFileValue()
		m.fileFilter = ""
		m.mode = modeNormal
		return m, m.restoreSelectedFile(selected, m.selected)
	case "enter":
		m.mode = modeNormal
		return m, m.ensureSelectedDiffCmd()
	case "backspace":
		if m.fileFilter == "" {
			return m, nil
		}
		selected := m.selectedFileValue()
		runes := []rune(m.fileFilter)
		m.fileFilter = string(runes[:len(runes)-1])
		return m, m.restoreSelectedFile(selected, m.selected)
	default:
		text := msg.Text
		if text == "" {
			text = msg.String()
		}
		runes := []rune(text)
		if len(runes) != 1 || runes[0] < 0x20 || runes[0] == 0x7f {
			return m, nil
		}
		selected := m.selectedFileValue()
		m.fileFilter += text
		return m, m.restoreSelectedFile(selected, 0)
	}
}

func (m *Model) handleMouse(mouse tea.Mouse) (bool, tea.Cmd) {
	if m.mode == modeThemePicker {
		overlay := m.themePicker.Render(m.width, m.bodyHeight(), m.styles, m.overlayPanelStyle())
		x, y := components.OverlayPosition(overlay, m.width, m.bodyHeight())
		if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) {
			return false, nil
		}
		next, cmd := m.handleThemePickerAction(m.themePicker.HandleMouse(mouse.Y-y, m.bodyHeight()))
		*m = next.(Model)
		return false, cmd
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
	index := m.fileListComponent().Offset(max(4, contentHeight-worktreeHeight)) + fileY - 1
	if index >= 0 && index < len(changes) {
		m.focusedPane = paneFiles
		previous := m.selectedFileValue()
		previousWorktree := m.selectedWorktreeValue().Path
		m.selected = index
		m.refreshDiff()
		return true, m.selectedDiffCmdAfterSelection(previous, previousWorktree)
	}
	return false, nil
}

func (m Model) handleThemeWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	overlay := m.themePicker.Render(m.width, m.bodyHeight(), m.styles, m.overlayPanelStyle())
	x, y := components.OverlayPosition(overlay, m.width, m.bodyHeight())
	if mouse.X < x || mouse.X >= x+lipgloss.Width(overlay) || mouse.Y < y || mouse.Y >= y+lipgloss.Height(overlay) {
		return m, nil
	}
	m.themePicker.HandleWheel(mouse.Button)
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
	return m.ensureSelectedDiffCmdWithYOffset(m.diff.YOffset())
}

func (m Model) ensureSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, false)
}

func (m Model) reloadSelectedDiffCmdWithYOffset(diffYOffset int) tea.Cmd {
	return m.selectedDiffCmdWithYOffset(diffYOffset, true)
}

func (m Model) selectedDiffCmdAfterSelection(previous gitview.FileChange, previousWorktreePath string) tea.Cmd {
	selected := m.selectedFileValue()
	if selected.Path != "" && (selected.Path != previous.Path || m.selectedWorktreeValue().Path != previousWorktreePath) {
		return m.reloadSelectedDiffCmdWithYOffset(m.diff.YOffset())
	}
	return m.ensureSelectedDiffCmd()
}

func (m Model) selectedDiffCmdWithYOffset(diffYOffset int, force bool) tea.Cmd {
	if m.loadDiff == nil {
		return nil
	}
	selected := m.selectedFileValue()
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
			diff:        m.loadDiff(m.context, worktreePath, selected),
			diffYOffset: diffYOffset,
		}
	}
}

func (m *Model) applySnapshot(snapshot Snapshot) (bool, int, bool) {
	selected := m.selectedFileValue()
	selectedWorktreePath := m.selectedWorktreeValue().Path
	selectedIndex := m.selected
	diffYOffset := m.diff.YOffset()
	preserveMergeTargetPicker := m.mode == modeMergeTarget
	mergeSource := m.merge.source
	mergeTargetPath := m.merge.selectedTargetPath()
	mergeRequest := m.merge.request
	preserveMergeConfirm := m.mode == modeMergeConfirm
	preserveMergeSubmitting := m.confirm.IsSubmitting()
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
		request, ok := m.refreshedMergeRequest(mergeRequest)
		if ok {
			m.mode = modeMergeConfirm
			m.merge.openConfirm(request)
			title, message := mergeConfirmText(request)
			m.confirm.Restore(title, message, preserveMergeSubmitting)
		} else {
			m.confirm.Close()
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
	preservedDiffScroll := selected.Path != "" && m.selectedFileValue().Path == selected.Path && m.selectedWorktreeValue().Path == selectedWorktreePath
	reloadSelectedDiff := snapshot.Diffs == nil && preservedDiffScroll && selectedDiffChanged(selected, m.selectedFileValue())
	if preservedDiffScroll {
		m.diff.SetYOffset(diffYOffset)
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
	targets := mergeTargets(refreshedSource, m.worktrees, m.defaultBranchName())
	if len(targets) == 0 {
		return false
	}
	m.merge.openTargetPicker(refreshedSource, targets, selectedTargetPath, m.styles, m.width, m.bodyHeight())
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
	m.diff.SetSize(max(10, components.FrameInnerWidth(rightWidth)), max(3, components.FrameInnerHeight(contentHeight)))
}

func (m *Model) refreshDiff() {
	m.resizeViewport()
	changes := m.visibleChanges()
	if len(changes) == 0 {
		if len(m.changes) > 0 && m.fileFilter != "" {
			m.diff.SetContent("empty:no-matching:"+m.fileFilter, "No matching files.")
			return
		}
		m.diff.SetContent("empty:no-changes:"+m.selectedWorktreeValue().Path, "No changes in this worktree.")
		return
	}
	m.selected = clamp(m.selected, 0, len(changes)-1)
	selected := changes[m.selected]
	diffContentKey := "diff:" + m.diffKey(selected)
	diff := m.selectedDiff()
	if diff == "" {
		diff = fmt.Sprintf("No diff loaded for %s", selected.Path)
	}
	if m.diff.SetContent(diffContentKey, diff) {
		m.diff.GotoTop()
	}
}

func (m Model) selectedDiff() string {
	selected := m.selectedFileValue()
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
	return m.diff.SetContent("", diff)
}

func (m Model) renderDiff(width, height int) string {
	selected := "[3]-" + components.IconFile + " Diff"
	if change := m.selectedFileValue(); change.Path != "" {
		selected = "[3]-" + change.Path
	}
	focused := m.focusedPane == paneDiff
	return m.diff.Render(selected, m.panel, focused, width, height)
}

func (m Model) fileOverlapBadge(change gitview.FileChange, background color.Color) string {
	count := len(m.overlapTargetsFor(change))
	if count == 0 {
		return ""
	}
	return listFillWithBackground(background, " ") +
		listStyleWithBackground(m.styles.Muted, background).Render(components.IconWarning) +
		listFillWithBackground(background, " ") +
		listStyleWithBackground(m.styles.Muted, background).Render(fmt.Sprintf("overlap %d", count))
}

func (m Model) overlayPanelStyle() lipgloss.Style {
	return m.styles.Panel
}

func (m Model) confirmWidth() int {
	if m.width <= 0 {
		return 64
	}
	return min(64, max(32, m.width-12))
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

func listStyleWithBackground(style lipgloss.Style, background color.Color) lipgloss.Style {
	return style.Background(background)
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

func changedFilesText(count int) string {
	if count == 1 {
		return "1 changed file"
	}
	return fmt.Sprintf("%d changed files", count)
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
	return components.RenderFileItem(styles, fileItem(change), filter, background, suffixParts...)
}

func fileItems(changes []gitview.FileChange) []components.FileItem {
	items := make([]components.FileItem, len(changes))
	for i, change := range changes {
		items[i] = fileItem(change)
	}
	return items
}

func fileItem(change gitview.FileChange) components.FileItem {
	return components.FileItem{
		ID:         change.Path,
		Path:       change.Path,
		StatusIcon: statusIcon(change.Status),
		Additions:  change.Additions,
		Deletions:  change.Deletions,
		Binary:     change.Binary,
	}
}

func statusIcon(status gitview.ChangeStatus) string {
	switch status {
	case gitview.Added:
		return components.IconAdded
	case gitview.Modified:
		return components.IconModified
	case gitview.Deleted:
		return components.IconDeleted
	case gitview.Renamed:
		return components.IconRenamed
	case gitview.Untracked:
		return components.IconUntracked
	default:
		return components.IconFile
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
