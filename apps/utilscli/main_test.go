package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestPipedInput verifies a utility accepts text from standard input.
func TestPipedInput(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"b64-decode"}, strings.NewReader("aGVsbG8="), &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "hello\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestUtilityTransforms covers representative deterministic utility results.
func TestUtilityTransforms(t *testing.T) {
	tests := []struct{ command, input, want string }{
		{"url-encode", "a!b*c()'", "a%21b%2Ac%28%29%27"},
		{"html-encode", `<a&'">`, "&lt;a&amp;&#39;&quot;&gt;"},
		{"json-minify", "{ \"a\": 1 }", "{\"a\":1}"},
		{"xml-minify", "<root>  <item>one</item> </root>", "<root><item>one</item></root>"},
	}
	for _, tt := range tests {
		got, err := execute(tt.command, tt.input)
		if err != nil {
			t.Fatalf("%s: %v", tt.command, err)
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.command, got, tt.want)
		}
	}
}

// TestPasswordAndUUIDDefaults ensures no-input generators use their defaults.
func TestPasswordAndUUIDDefaults(t *testing.T) {
	for _, command := range []string{"password", "uuid"} {
		var out bytes.Buffer
		if err := run([]string{command}, strings.NewReader(""), &out); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if strings.TrimSpace(out.String()) == "" {
			t.Errorf("%s generated no value", command)
		}
	}
}

// TestInteractiveMenu verifies the terminal UI includes its main panels.
func TestInteractiveMenu(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	drawTUI(screen, &tuiState{})
	r, _, _, _ := screen.GetContent(3, 0)
	if r != 'T' {
		t.Fatalf("tools panel was not rendered, got %q", r)
	}
}

func TestInteractiveOutputCanScroll(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 20)

	state := tuiState{result: strings.Join([]string{
		"line 00", "line 01", "line 02", "line 03", "line 04",
		"line 05", "line 06", "line 07", "line 08", "line 09",
	}, "\n")}
	drawTUI(screen, &state)
	if got := simulationText(screen, 36, 13, 7); got != "line 00" {
		t.Fatalf("initial output row = %q, want %q", got, "line 00")
	}

	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	drawTUI(screen, &state)
	if got := simulationText(screen, 36, 13, 7); got == "line 00" {
		t.Fatalf("output did not scroll after Page Down; first row is still %q", got)
	}
}

func TestOutputScrollClampsToContent(t *testing.T) {
	state := tuiState{
		focus:        focusOutput,
		result:       "one\ntwo\nthree\nfour\nfive",
		outputWidth:  20,
		outputHeight: 3,
	}
	for range 10 {
		handleTUIKey(&state, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	if got, want := state.outputOffset, 2; got != want {
		t.Fatalf("output offset = %d, want clamped offset %d", got, want)
	}
	for range 10 {
		handleTUIKey(&state, tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	}
	if state.outputOffset != 0 {
		t.Fatalf("output offset = %d, want 0", state.outputOffset)
	}
}

func TestShortOutputDoesNotScroll(t *testing.T) {
	state := tuiState{focus: focusOutput, result: "one line", outputWidth: 20, outputHeight: 3}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if state.outputOffset != 0 {
		t.Fatalf("short output offset = %d, want 0", state.outputOffset)
	}
}

func TestRunningToolResetsOutputScroll(t *testing.T) {
	state := tuiState{input: []rune("hello"), outputOffset: 8}
	runSelectedTool(&state)
	if state.outputOffset != 0 {
		t.Fatalf("output offset = %d after running tool, want 0", state.outputOffset)
	}
}

func TestCopyOutputToClipboard(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	original := writeSystemClipboard
	defer func() { writeSystemClipboard = original }()
	var systemClipboard string
	writeSystemClipboard = func(value string) error {
		systemClipboard = value
		return nil
	}
	state := tuiState{result: "copy me"}
	handleTUIEvent(screen, &state, tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if got, want := string(screen.GetClipboardData()), state.result; got != want {
		t.Fatalf("terminal clipboard = %q, want %q", got, want)
	}
	if got, want := systemClipboard, state.result; got != want {
		t.Fatalf("system clipboard = %q, want %q", got, want)
	}
	if state.status != "Output copied" {
		t.Fatalf("copy status = %q, want success", state.status)
	}
}

func TestCopyStatusIsRenderedInOutputPane(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 20)
	state := tuiState{result: "copy me", status: "Output copied"}
	drawTUI(screen, &state)
	if got, want := simulationText(screen, 36, 13, 15), "✓ Output copied"; got != want {
		t.Fatalf("copy notification = %q, want %q", got, want)
	}
}

func TestCopyOutputReportsSystemClipboardFailure(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	original := writeSystemClipboard
	defer func() { writeSystemClipboard = original }()
	writeSystemClipboard = func(string) error { return errors.New("clipboard unavailable") }
	state := tuiState{result: "copy me"}
	handleTUIEvent(screen, &state, tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if got := string(screen.GetClipboardData()); got != state.result {
		t.Fatalf("terminal clipboard fallback = %q, want %q", got, state.result)
	}
	if !strings.Contains(state.status, "terminal") {
		t.Fatalf("copy failure status = %q, want terminal fallback notice", state.status)
	}
}

func TestCopyWithEmptyOutputIsNoOp(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	original := writeSystemClipboard
	defer func() { writeSystemClipboard = original }()
	called := false
	writeSystemClipboard = func(string) error {
		called = true
		return nil
	}
	state := tuiState{}
	handleTUIEvent(screen, &state, tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if called {
		t.Fatal("system clipboard was called for empty output")
	}
	if state.status != "No output to copy" {
		t.Fatalf("empty copy status = %q, want no-output notice", state.status)
	}
}

func TestMouseWheelScrollsOutput(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	state := tuiState{result: "one\ntwo\nthree\nfour", outputWidth: 20, outputHeight: 2}
	handleTUIEvent(screen, &state, tcell.NewEventMouse(0, 0, tcell.WheelDown, tcell.ModNone))
	if got, want := state.outputOffset, 2; got != want {
		t.Fatalf("mouse wheel output offset = %d, want %d", got, want)
	}
}

func TestTabCyclesThroughOutput(t *testing.T) {
	state := initialTUIState()
	advanceFocus(&state)
	advanceFocus(&state)
	if state.focus != focusOutput {
		t.Fatalf("focus = %v after two tabs, want output", state.focus)
	}
	advanceFocus(&state)
	if state.focus != focusTools {
		t.Fatalf("focus = %v after three tabs, want tools", state.focus)
	}
}

func simulationText(screen tcell.SimulationScreen, x, y, width int) string {
	runes := make([]rune, 0, width)
	for column := x; column < x+width; column++ {
		char, _, _, _ := screen.GetContent(column, y)
		runes = append(runes, char)
	}
	return string(runes)
}

func TestInitialTUIFocusIsTools(t *testing.T) {
	state := initialTUIState()
	if state.focus != focusTools {
		t.Fatalf("initial focus = %v, want tools panel", state.focus)
	}
}

func TestActiveFocusHintUsesDistinctColor(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	drawTUI(screen, &tuiState{focus: focusTools})

	_, _, titleStyle, _ := screen.GetContent(3, 0)
	_, _, hintStyle, _ := screen.GetContent(10, 0)
	if titleStyle == hintStyle {
		t.Fatal("active focus hint should use a distinct style")
	}
}

func TestEditorCursorNavigation(t *testing.T) {
	state := tuiState{input: []rune("one\ntwo"), cursor: 1, inputWidth: 20, focus: focusInput}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if state.cursor != 2 {
		t.Fatalf("right arrow moved cursor to %d, want 2", state.cursor)
	}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if state.cursor != 6 {
		t.Fatalf("down arrow moved cursor to %d, want 6", state.cursor)
	}
}

func TestEditorEnterAddsNewline(t *testing.T) {
	state := tuiState{input: []rune("first"), cursor: 5, focus: focusInput}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if got, want := string(state.input), "first\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSwitchingToolsClearsInputAndResult(t *testing.T) {
	state := tuiState{
		selected: 0,
		input:    []rune("old input"),
		cursor:   3,
		focus:    focusTools,
		result:   "old result",
	}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if state.selected != 1 {
		t.Fatalf("selected = %d, want 1", state.selected)
	}
	if len(state.input) != 0 || state.cursor != 0 || state.result != "" {
		t.Fatalf("tool state was not cleared: %#v", state)
	}
}

func TestTUIToolsAreCommands(t *testing.T) {
	for _, tool := range tuiTools {
		if canonicalCommand(tool.command) == "" {
			t.Errorf("%q is not a supported command", tool.command)
		}
	}
}

func TestUnknownCommandDoesNotUsePromptRouting(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"base64 hello"}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected an unknown command error")
	}
}
