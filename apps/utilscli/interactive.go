package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type toolDefinition struct {
	command     string
	description string
}

var tuiTools = []toolDefinition{
	{"b64-encode", "Encode text as Base64"},
	{"b64-decode", "Decode Base64 text"},
	{"b64url-encode", "Encode URL-safe Base64"},
	{"b64url-decode", "Decode URL-safe Base64"},
	{"url-encode", "Percent-encode a URL component"},
	{"url-decode", "Decode a URL component"},
	{"html-encode", "Escape HTML special characters"},
	{"html-decode", "Decode HTML entities"},
	{"json-pretty", "Validate and format JSON"},
	{"json-minify", "Validate and minify JSON"},
	{"xml-pretty", "Validate and format XML"},
	{"xml-minify", "Validate and minify XML"},
	{"jwt", "Decode a JWT"},
	{"saml", "Decode a SAML message"},
	{"hash", "Generate common hashes"},
	{"uuid", "Generate a UUID"},
	{"password", "Generate a password"},
	{"timestamp", "Convert a timestamp or date"},
}

type tuiState struct {
	selected     int
	input        []rune
	cursor       int
	inputWidth   int
	focus        tuiFocus
	result       string
	outputOffset int
	outputWidth  int
	outputHeight int
	status       string
}

type tuiFocus int

const (
	focusInput tuiFocus = iota
	focusTools
	focusOutput
)

type toolInputMode int

const (
	inputText toolInputMode = iota
	inputNone
)

func selectedInputMode(state *tuiState) toolInputMode {
	switch tuiTools[state.selected].command {
	case "uuid", "password":
		return inputNone
	default:
		return inputText
	}
}

func initialTUIState() tuiState {
	return tuiState{focus: focusTools}
}

// runInteractive opens a full-screen terminal UI for a bare uc invocation.
func runInteractive(in io.Reader, out io.Writer) error {
	inFile, inputIsFile := in.(*os.File)
	outFile, outputIsFile := out.(*os.File)
	if !inputIsFile || !outputIsFile || !isTerminal(inFile) || !isTerminal(outFile) {
		return fmt.Errorf("the interactive UI requires a terminal; run uc from a terminal window or use 'uc --help'")
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal UI: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("start terminal UI: %w", err)
	}
	defer screen.Fini()
	screen.EnableMouse()
	defer screen.DisableMouse()

	state := initialTUIState()
	for {
		drawTUI(screen, &state)
		if handleTUIEvent(screen, &state, screen.PollEvent()) {
			return nil
		}
		for screen.HasPendingEvent() {
			if handleTUIEvent(screen, &state, screen.PollEvent()) {
				return nil
			}
		}
	}
}

// handleTUIEvent applies queued input before the next render, which keeps long
// pastes responsive without relying on terminal-specific bracketed-paste events.
func handleTUIEvent(screen tcell.Screen, state *tuiState, event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventResize:
		screen.Sync()
	case *tcell.EventKey:
		if event.Key() == tcell.KeyCtrlY {
			copyOutput(screen, state)
			return false
		}
		return handleTUIKey(state, event)
	case *tcell.EventMouse:
		switch event.Buttons() {
		case tcell.WheelUp:
			scrollOutput(state, -3)
		case tcell.WheelDown:
			scrollOutput(state, 3)
		}
	}
	return false
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// handleTUIKey updates the UI and reports whether it should exit.
func handleTUIKey(state *tuiState, event *tcell.EventKey) bool {
	state.status = ""
	switch event.Key() {
	case tcell.KeyCtrlC, tcell.KeyEscape:
		return true
	case tcell.KeyTAB:
		advanceFocus(state)
	case tcell.KeyUp:
		if state.focus == focusOutput {
			scrollOutput(state, -1)
		} else if state.focus == focusTools && state.selected > 0 {
			selectTool(state, state.selected-1)
		} else if input, cursor := activeInput(state); input != nil {
			*cursor = moveCursorVertical(*input, *cursor, -1, state.inputWidth)
		}
	case tcell.KeyDown:
		if state.focus == focusOutput {
			scrollOutput(state, 1)
		} else if state.focus == focusTools && state.selected < len(tuiTools)-1 {
			selectTool(state, state.selected+1)
		} else if input, cursor := activeInput(state); input != nil {
			*cursor = moveCursorVertical(*input, *cursor, 1, state.inputWidth)
		}
	case tcell.KeyPgUp:
		state.focus = focusOutput
		scrollOutput(state, -max(1, state.outputHeight))
	case tcell.KeyPgDn:
		state.focus = focusOutput
		scrollOutput(state, max(1, state.outputHeight))
	case tcell.KeyHome:
		if state.focus == focusOutput {
			state.outputOffset = 0
		}
	case tcell.KeyEnd:
		if state.focus == focusOutput {
			state.outputOffset = maxOutputOffset(state)
		}
	case tcell.KeyLeft:
		if _, cursor := activeInput(state); cursor != nil && *cursor > 0 {
			*cursor--
		}
	case tcell.KeyRight:
		if input, cursor := activeInput(state); input != nil && *cursor < len(*input) {
			*cursor++
		}
	case tcell.KeyEnter:
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, '\n')
		} else if selectedInputMode(state) == inputNone {
			runSelectedTool(state)
		} else {
			state.focus = firstInputFocus(selectedInputMode(state))
		}
	case tcell.KeyCtrlR:
		runSelectedTool(state)
	case tcell.KeyCtrlJ:
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, '\n')
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if input, cursor := activeInput(state); input != nil && *cursor > 0 {
			*input = append((*input)[:*cursor-1], (*input)[*cursor:]...)
			*cursor--
		}
	case tcell.KeyRune:
		if state.focus == focusTools && selectedInputMode(state) != inputNone {
			state.focus = firstInputFocus(selectedInputMode(state))
		}
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, event.Rune())
		}
	}
	return false
}

func selectTool(state *tuiState, selected int) {
	if selected == state.selected {
		return
	}
	state.selected = selected
	clearToolState(state)
}

func clearToolState(state *tuiState) {
	state.input = nil
	state.cursor = 0
	state.result = ""
	state.outputOffset = 0
	state.status = ""
}

func advanceFocus(state *tuiState) {
	switch selectedInputMode(state) {
	case inputNone:
		if state.focus == focusTools {
			state.focus = focusOutput
		} else {
			state.focus = focusTools
		}
	default:
		switch state.focus {
		case focusTools:
			state.focus = focusInput
		case focusInput:
			state.focus = focusOutput
		default:
			state.focus = focusTools
		}
	}
}

func firstInputFocus(mode toolInputMode) tuiFocus {
	return focusInput
}

func activeInput(state *tuiState) (*[]rune, *int) {
	switch state.focus {
	case focusInput:
		return &state.input, &state.cursor
	default:
		return nil, nil
	}
}

func insertInputRune(input *[]rune, cursor *int, char rune) {
	*input = append(*input, 0)
	copy((*input)[*cursor+1:], (*input)[*cursor:])
	(*input)[*cursor] = char
	*cursor++
}

func runSelectedTool(state *tuiState) {
	tool := tuiTools[state.selected]
	input := string(state.input)
	state.outputOffset = 0
	state.status = ""
	result, err := execute(tool.command, input)
	if err != nil {
		state.result = "Error: " + err.Error()
		return
	}
	state.result = result
}

func drawTUI(screen tcell.Screen, state *tuiState) {
	width, height := screen.Size()
	screen.Clear()
	if width < 60 || height < 14 {
		drawText(screen, 0, 0, width, "Please enlarge the terminal to at least 60×14.", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		screen.Show()
		return
	}

	leftWidth := width / 3
	if leftWidth < 25 {
		leftWidth = 25
	}
	rightX := leftWidth + 1
	rightWidth := width - rightX
	state.inputWidth = rightWidth - 4

	boxStyle := tcell.StyleDefault.Foreground(tcell.ColorMediumPurple)
	titleStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
	activeStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorWhite).Background(tcell.ColorDarkCyan)
	mutedStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	drawBox(screen, 0, 0, leftWidth, height-2, "Tools", state.focus == focusTools, boxStyle, activeStyle)
	drawBox(screen, rightX, 0, rightWidth, height-2, tuiTools[state.selected].command, false, boxStyle, activeStyle)

	drawTools(screen, *state, 1, 1, leftWidth-2, height-4)
	drawText(screen, rightX+2, 2, rightWidth-4, tuiTools[state.selected].description, mutedStyle)
	switch selectedInputMode(state) {
	case inputNone:
		drawNoInputView(screen, state, rightX+2, rightWidth-4, height, titleStyle, activeStyle, mutedStyle)
	default:
		drawTextInputView(screen, state, rightX+2, rightWidth-4, height, titleStyle, activeStyle)
	}
	drawText(screen, 0, height-1, width, "Tab switches panes · PgUp/PgDn scroll output · Ctrl+Y copies · Ctrl+R runs · Esc exits", mutedStyle)
	screen.Show()
}

func drawTools(screen tcell.Screen, state tuiState, x, y, width, height int) {
	offset := 0
	if state.selected >= height {
		offset = state.selected - height + 1
	}
	for row := 0; row < height; row++ {
		index := row + offset
		if index >= len(tuiTools) {
			return
		}
		style := tcell.StyleDefault
		prefix := "  "
		if index == state.selected {
			style = style.Foreground(tcell.ColorBlack).Background(tcell.ColorAqua).Bold(true)
			prefix = "› "
		}
		drawText(screen, x, y+row, width, prefix+tuiTools[index].command, style)
	}
}

func drawTextInputView(screen tcell.Screen, state *tuiState, x, width, screenHeight int, titleStyle, activeStyle tcell.Style) {
	inputHeight := max(3, screenHeight/3)
	drawTitle(screen, x, 4, width, "Input", state.focus == focusInput, titleStyle, activeStyle)
	drawInput(screen, state.input, state.cursor, state.focus == focusInput, x, 5, width, inputHeight, "Type input here")
	drawOutput(screen, state, x, 6+inputHeight, width, screenHeight-(9+inputHeight), titleStyle, activeStyle)
}

func drawNoInputView(screen tcell.Screen, state *tuiState, x, width, screenHeight int, titleStyle, activeStyle, mutedStyle tcell.Style) {
	drawText(screen, x, 4, width, "No input required", titleStyle)
	drawText(screen, x, 5, width, "Press Enter or Ctrl+R to generate a result.", mutedStyle)
	drawOutput(screen, state, x, 7, width, screenHeight-10, titleStyle, activeStyle)
}

func drawOutput(screen tcell.Screen, state *tuiState, x, y, width, height int, titleStyle, activeStyle tcell.Style) {
	contentY := y + 1
	contentHeight := height
	if state.status != "" {
		statusStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow)
		prefix := "! "
		if state.status == "Output copied" {
			statusStyle = tcell.StyleDefault.Bold(true).Foreground(tcell.ColorGreen)
			prefix = "✓ "
		}
		drawText(screen, x, contentY, width, prefix+state.status, statusStyle)
		contentY++
		contentHeight = max(0, contentHeight-1)
	}
	state.outputWidth = width
	state.outputHeight = max(0, contentHeight)
	state.outputOffset = min(max(state.outputOffset, 0), maxOutputOffset(state))
	title := "Output"
	lineCount := len(wrapTUI(state.result, width))
	if lineCount > contentHeight && contentHeight > 0 {
		title = fmt.Sprintf("Output [%d–%d/%d]", state.outputOffset+1, min(state.outputOffset+contentHeight, lineCount), lineCount)
	}
	drawTitle(screen, x, y, width, title, state.focus == focusOutput, titleStyle, activeStyle)
	result := state.result
	if result == "" {
		result = "Output appears here after you run the tool."
	}
	drawLines(screen, x, contentY, width, contentHeight, state.outputOffset, result, tcell.StyleDefault)
}

func scrollOutput(state *tuiState, delta int) {
	state.outputOffset = min(max(state.outputOffset+delta, 0), maxOutputOffset(state))
}

func copyOutput(screen tcell.Screen, state *tuiState) {
	if state.result == "" {
		state.status = "No output to copy"
		return
	}
	screen.SetClipboard([]byte(state.result))
	if err := writeSystemClipboard(state.result); err != nil {
		state.status = "Copy sent to terminal; clipboard access may be blocked"
		return
	}
	state.status = "Output copied"
}

func maxOutputOffset(state *tuiState) int {
	if state.outputWidth < 1 || state.outputHeight < 1 || state.result == "" {
		return 0
	}
	return max(0, len(wrapTUI(state.result, state.outputWidth))-state.outputHeight)
}

func drawInput(screen tcell.Screen, input []rune, cursor int, active bool, x, y, width, height int, placeholder string) {
	lines := wrapTUI(string(input), width)
	cursorLine, cursorColumn := inputCursorPosition(input, cursor, width)
	if cursorLine >= len(lines) {
		lines = append(lines, "")
	}
	firstLine := max(0, cursorLine-height+1)
	if len(input) == 0 {
		drawText(screen, x, y, width, placeholder, tcell.StyleDefault.Foreground(tcell.ColorGray))
	}
	for row := 0; row < height; row++ {
		lineIndex := firstLine + row
		if lineIndex >= len(lines) {
			break
		}
		drawText(screen, x, y+row, width, lines[lineIndex], tcell.StyleDefault)
	}
	if !active || cursorLine < firstLine || cursorLine >= firstLine+height {
		return
	}
	line := []rune(lines[cursorLine])
	cursorChar := ' '
	if cursorColumn < len(line) {
		cursorChar = line[cursorColumn]
	}
	screen.SetContent(x+cursorColumn, y+cursorLine-firstLine, cursorChar, nil, tcell.StyleDefault.Reverse(true))
}

func inputCursorPosition(input []rune, cursor, width int) (int, int) {
	if width < 1 {
		width = 1
	}
	cursor = min(max(cursor, 0), len(input))
	line, column := 0, 0
	for _, char := range input[:cursor] {
		if char == '\n' {
			line, column = line+1, 0
			continue
		}
		column++
		if column == width {
			line, column = line+1, 0
		}
	}
	return line, column
}

func moveCursorVertical(input []rune, cursor, direction, width int) int {
	currentLine, currentColumn := inputCursorPosition(input, cursor, width)
	lastLine, _ := inputCursorPosition(input, len(input), width)
	targetLine := min(max(currentLine+direction, 0), lastLine)
	bestPosition, bestDistance := cursor, int(^uint(0)>>1)
	for position := 0; position <= len(input); position++ {
		line, column := inputCursorPosition(input, position, width)
		if line != targetLine {
			continue
		}
		distance := column - currentColumn
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestPosition, bestDistance = position, distance
		}
	}
	return bestPosition
}

func drawBox(screen tcell.Screen, x, y, width, height int, title string, active bool, style, activeStyle tcell.Style) {
	if width < 2 || height < 2 {
		return
	}
	for column := x; column < x+width; column++ {
		screen.SetContent(column, y, '─', nil, style)
		screen.SetContent(column, y+height-1, '─', nil, style)
	}
	for row := y; row < y+height; row++ {
		screen.SetContent(x, row, '│', nil, style)
		screen.SetContent(x+width-1, row, '│', nil, style)
	}
	screen.SetContent(x, y, '┌', nil, style)
	screen.SetContent(x+width-1, y, '┐', nil, style)
	screen.SetContent(x, y+height-1, '└', nil, style)
	screen.SetContent(x+width-1, y+height-1, '┘', nil, style)
	drawTitle(screen, x+2, y, width-4, " "+title, active, style.Bold(true), activeStyle)
}

func drawTitle(screen tcell.Screen, x, y, width int, title string, active bool, titleStyle, activeStyle tcell.Style) {
	drawText(screen, x, y, width, title, titleStyle)
	if !active {
		return
	}
	offset := len([]rune(title))
	drawText(screen, x+offset, y, width-offset, " (active · Tab switches focus)", activeStyle)
}

func drawLines(screen tcell.Screen, x, y, width, height, offset int, value string, style tcell.Style) {
	lines := wrapTUI(value, width)
	if len(lines) == 0 {
		drawText(screen, x, y, width, "Type input here", style.Foreground(tcell.ColorGray))
		return
	}
	for row, line := range lines[offset:] {
		if row >= height {
			return
		}
		drawText(screen, x, y+row, width, line, style)
	}
}

func drawText(screen tcell.Screen, x, y, width int, value string, style tcell.Style) {
	if width <= 0 {
		return
	}
	for index, char := range []rune(value) {
		if index >= width {
			return
		}
		screen.SetContent(x+index, y, char, nil, style)
	}
}

func wrapTUI(value string, width int) []string {
	if value == "" || width < 1 {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		runes := []rune(line)
		if len(runes) == 0 {
			lines = append(lines, "")
		}
		for len(runes) > 0 {
			end := min(width, len(runes))
			lines = append(lines, string(runes[:end]))
			runes = runes[end:]
		}
	}
	return lines
}
