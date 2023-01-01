package readline

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// History is an interface to allow you to write your own history logging
// tools. eg sqlite backend instead of a file system.
// By default readline will just use the dummyLineHistory interface which only
// logs the history to memory ([]string to be precise).
type History interface {
	// Append takes the line and returns an updated number of lines or an error
	Write(string) (int, error)

	// GetLine takes the historic line number and returns the line or an error
	GetLine(int) (string, error)

	// Len returns the number of history lines
	Len() int

	// Dump returns everything in readline. The return is an interface{} because
	// not all LineHistory implementations will want to structure the history in
	// the same way. And since Dump() is not actually used by the readline API
	// internally, this methods return can be structured in whichever way is most
	// convenient for your own applications (or even just create an empty
	// function which returns `nil` if you don't require Dump() either)
	Dump() interface{}
}

// AddHistorySource adds a source of history lines bound to a given name
// (printed above this source when used). All history sources can be used
// when completing history, and repeatedly using the completion key will
// cycle through them.
// When only the default in-memory history is bound, it is replaced with
// the provided ones. All following sources are added to the list.
func (rl *Instance) AddHistorySource(name string, history History) {
	if len(rl.histories) == 1 && rl.historyNames[0] == "local history" {
		delete(rl.histories, "local history")
		rl.historyNames = make([]string, 0)
	}

	rl.historyNames = append(rl.historyNames, name)
	rl.histories[name] = history
}

// New creates a new History populated from, and writing to
// a file path passed as parameter.
func NewHistoryFromFile(filename string) (History, error) {
	h := new(fileHistory)
	h.filename = filename
	h.list, _ = openHist(filename)

	return h, nil
}

// DeleteHistorySource deletes one or more history source by name.
// If no arguments are passed, all currently bound sources are removed.
func (rl *Instance) DeleteHistorySource(sources ...string) {
	defer rl.initHistory()

	if len(sources) == 0 {
		rl.histories = make(map[string]History)
		rl.historyNames = make([]string, 0)

		return
	}

	for _, name := range sources {
		delete(rl.histories, name)
		for i, hname := range rl.historyNames {
			if hname == name {
				rl.historyNames = append(rl.historyNames[:i], rl.historyNames[i+1:]...)
				break
			}
		}
	}
}

// MemoryHistory is an in memory history.
// One such history is bound to the readline shell by default.
type MemoryHistory struct {
	items []string
}

// Write to history.
func (h *MemoryHistory) Write(s string) (int, error) {
	h.items = append(h.items, s)
	return len(h.items), nil
}

// GetLine returns a line from history.
func (h *MemoryHistory) GetLine(i int) (string, error) {
	return h.items[i], nil
}

// Len returns the number of lines in history.
func (h *MemoryHistory) Len() int {
	return len(h.items)
}

// Dump returns the entire history.
func (h *MemoryHistory) Dump() interface{} {
	return h.items
}

// NullHistory is a null History interface for when you don't want line
// entries remembered eg password input.
type NullHistory struct{}

// Write to history.
func (h *NullHistory) Write(s string) (int, error) {
	return 0, nil
}

// GetLine returns a line from history.
func (h *NullHistory) GetLine(i int) (string, error) {
	return "", nil
}

// Len returns the number of lines in history.
func (h *NullHistory) Len() int {
	return 0
}

// Dump returns the entire history.
func (h *NullHistory) Dump() interface{} {
	return []string{}
}

// initHistory is ran once at the beginning of an instance start.
func (rl *Instance) initHistory() {
	rl.historySourcePos = 0
	rl.undoHistory = []undoItem{{line: "", pos: 0}}

	// Only reset the history position when we don't
	// need it to retrieve a line before anything else.
	if !rl.inferLine {
		rl.histPos = 0
	}
}

// when the last widget invoked accepted a line with a supplementary
// directive to retrieve a history line (by match or index), find it.
func (rl *Instance) initHistoryLine() {
	if !rl.inferLine {
		return
	}

	switch rl.histPos {
	case -1:
		rl.histPos = 0
	case 0:
		rl.inferNextHistory()
	default:
		rl.walkHistory(-1)
	}

	rl.inferLine = false
}

func (rl *Instance) nextHistorySource() {
	rl.historySourcePos++

	if rl.historySourcePos == len(rl.historyNames) {
		rl.historySourcePos = 0
	}
}

func (rl *Instance) prevHistorySource() {
	rl.historySourcePos--

	if rl.historySourcePos < 0 {
		rl.historySourcePos = len(rl.historyNames) - 1
	}
}

func (rl *Instance) currentHistory() History {
	if len(rl.histories) == 0 {
		return nil
	}
	return rl.histories[rl.historyNames[rl.historySourcePos]]
}

// walkHistory - Browse historic lines.
func (rl *Instance) walkHistory(pos int) {
	var (
		next string
		err  error
	)

	// Always use the main/first history.
	rl.historySourcePos = 0
	history := rl.currentHistory()

	if history == nil || history.Len() == 0 {
		return
	}

	// When we are exiting the current line buffer to move around
	// the history, we make buffer the current line
	if rl.histPos == 0 && (rl.histPos+pos) == 1 {
		rl.lineBuf = string(rl.line)
	}

	rl.histPos += pos

	switch {
	case rl.histPos > history.Len():
		// The history is greater than the length of history: maintain
		// it at the last index, to keep the same line in the buffer.
		rl.histPos = history.Len()
	case rl.histPos < 0:
		// We can never go lower than the last history line, which is our current line.
		rl.histPos = 0
	case rl.histPos == 0:
		// The 0 index is our current line
		rl.line = []rune(rl.lineBuf)
		rl.pos = len(rl.lineBuf)
	}

	// We now have the correct history index. Use it to find the history line.
	// If the history position is not zero, we need to use a history line.
	if rl.histPos > 0 {
		next, err = history.GetLine(history.Len() - rl.histPos)
		if err != nil {
			rl.resetHelpers()
			print("\r\n" + err.Error() + "\r\n")
			// NOTE: Same here ? Should we print the prompt more properly ?
			print(rl.Prompt.primary)
			return
		}

		rl.clearLine()
		rl.line = []rune(next)
		rl.pos = len(rl.line)
	}
}

// completeHistory - Populates a CompletionGroup with history and returns it the shell
// we populate only one group, so as to pass it to the main completion engine.
func (rl *Instance) completeHistory(forward bool) Completions {
	if len(rl.histories) == 0 {
		return Completions{}
	}

	history := rl.currentHistory()
	if history == nil {
		return Completions{}
	}

	rl.histHint = []rune(rl.historyNames[rl.historySourcePos])

	// Set the hint line with everything
	rl.histHint = []rune(seqBold + seqFgCyanBright + string(rl.histHint) + seqReset)

	compLines := make([]Completion, 0)

	// Set up iteration clauses
	var histPos int
	var done func(i int) bool
	var move func(inc int) int

	if forward {
		histPos = -1
		done = func(i int) bool { return i < history.Len() }
		move = func(pos int) int { return pos + 1 }
	} else {
		histPos = history.Len()
		done = func(i int) bool { return i > 0 }
		move = func(pos int) int { return pos - 1 }
	}

	// And generate the completions.
NEXT_LINE:
	for done(histPos) {
		histPos = move(histPos)

		line, err := history.GetLine(histPos)
		if err != nil {
			continue
		}

		if !strings.HasPrefix(line, rl.tcPrefix) || strings.TrimSpace(line) == "" {
			continue
		}

		line = strings.ReplaceAll(line, "\n", ` `)

		for _, comp := range compLines {
			if comp.Display == line {
				continue NEXT_LINE
			}
		}

		// Proper pad for indexes
		indexStr := strconv.Itoa(histPos)
		pad := strings.Repeat(" ", len(strconv.Itoa(history.Len()))-len(indexStr))
		display := fmt.Sprintf("%s%s %s%s", seqDim, indexStr+pad, seqDimReset, line)

		value := Completion{
			Display: display,
			Value:   line,
		}

		compLines = append(compLines, value)

	}

	comps := CompleteRaw(compLines)
	comps = comps.NoSort()
	comps.PREFIX = string(rl.line)

	return comps
}

// fileHistory provides a history source based on a file.
type fileHistory struct {
	filename string
	list     []string
}

func openHist(filename string) (list []string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return list, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		list = append(list, scanner.Text())
	}

	file.Close()
	return list, nil
}

// Write item to history file. eg ~/.murex_history.
func (h *fileHistory) Write(s string) (int, error) {
	block := strings.TrimSpace(s)

	if len(h.list) == 0 || h.list[len(h.list)-1] != block {
		h.list = append(h.list, block)
	}

	f, err := os.OpenFile(h.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}

	_, err = f.WriteString(block + "\n")
	f.Close()

	return h.Len(), err
}

// GetLine returns a specific line from the history file.
func (h *fileHistory) GetLine(i int) (string, error) {
	if i < 0 {
		return "", errors.New("cannot use a negative index when requesting historic commands")
	}
	if i < len(h.list) {
		return h.list[i], nil
	}
	return "", errors.New("index requested greater than number of items in history")
}

// Len returns the number of items in the history file.
func (h *fileHistory) Len() int {
	return len(h.list)
}

// Dump returns the entire history file.
func (h *fileHistory) Dump() interface{} {
	return h.list
}
