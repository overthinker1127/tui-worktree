package components

func FrameInnerWidth(width int) int {
	return max(0, width-2)
}

func FrameInnerHeight(height int) int {
	return max(0, height-2)
}
