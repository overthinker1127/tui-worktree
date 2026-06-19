package tui

type uiMode int

const (
	modeNormal uiMode = iota
	modeFileFilter
	modeThemePicker
	modeDeleteConfirm
	modePRForm
	modeMergeTarget
	modeMergeConfirm
	modeOverlapPicker
	modeOverlapCompare
)

func (mode uiMode) blocksDiffWheel() bool {
	switch mode {
	case modeFileFilter, modeDeleteConfirm, modePRForm, modeMergeTarget, modeMergeConfirm, modeOverlapPicker:
		return true
	default:
		return false
	}
}
