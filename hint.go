package readline

import (
	"regexp"
	"strings"

	ansi "github.com/acarl005/stripansi"
)

// SetHint - a nasty function to force writing a new hint text.
// It does not update helpers, it just renders them, so the hint
// will survive until the helpers (thus including the hint) will
// be updated/recomputed.
func (rl *Instance) SetHint(s string) {
	rl.hint = []rune(s)
	rl.renderHelpers()
}

func (rl *Instance) getHintText() {
	// Before entering this function, some completer might have
	// been called, which might have already populated the hint
	// area (with either an error, a usage, etc).
	// Some of the branchings below will overwrite it.

	// Use the user-provided hint by default, if not empty.
	if rl.HintText != nil {
		userHint := rl.HintText(rl.getCompletionLine())
		if len(userHint) > 0 {
			rl.hint = rl.HintText(rl.getCompletionLine())
		}
	}

	// When completing history, we have a special hint
	if len(rl.histHint) > 0 {
		rl.hint = append([]rune{}, rl.histHint...)
	}

	// But the local keymap, especially completions,
	// overwrites the user hint, since either:
	// - We have some completions, which are self-describing
	// - We didn't match any, thus we have a new error hint.
	switch rl.local {
	case isearch:
		rl.isearchHint()
	case menuselect:
		if rl.noCompletions() {
			rl.hintNoMatches()
		}
	}
}

// hintCompletions generates a hint string from all the
// usage/message strings yielded by completions.
func (rl *Instance) hintCompletions(comps Completions) {
	rl.hint = []rune{}

	// First add the command/flag usage string if any,
	// and only if we don't have completions.
	if len(comps.values) == 0 {
		rl.hint = append([]rune(seqDim), []rune(comps.usage)...)
	}

	// And all further messages
	for _, message := range comps.messages.Get() {
		if message == "" {
			continue
		}

		rl.hint = append(rl.hint, []rune(message+"\n")...)
	}

	// Remove the last newline
	if len(rl.hint) > 0 && rl.hint[len(rl.hint)-1] == '\n' {
		rl.hint = rl.hint[:len(rl.hint)-2]
	}

	// And remove the coloring if no hint
	if string(rl.hint) == seqDim {
		rl.hint = []rune{}
	}
}

// generate a hint when no completion matches the prefix.
func (rl *Instance) hintNoMatches() {
	noMatches := seqDim + "no matching "

	var groups []string
	for _, group := range rl.tcGroups {
		if group.tag == "" {
			continue
		}
		groups = append(groups, group.tag)
	}

	// History has no named group, so add it
	if len(groups) == 0 && len(rl.histHint) > 0 {
		groups = append(groups, rl.historyNames[rl.historySourcePos])
	}

	if len(groups) > 0 {
		groupsStr := strings.Join(groups, ", ")
		noMatches += "'" + groupsStr + "'"
	}

	rl.hint = []rune(noMatches + " completions")
}

// writeHintText - only writes the hint text and computes its offsets.
func (rl *Instance) writeHintText() {
	if len(rl.hint) == 0 {
		rl.hintY = 0
		return
	}

	// Wraps the line, and counts the number of newlines
	// in the string, adjusting the offset as well.
	re := regexp.MustCompile(`\r?\n`)
	newlines := re.Split(string(rl.hint), -1)
	offset := len(newlines)

	_, actual := wrapText(ansi.Strip(string(rl.hint)), GetTermWidth())
	wrapped, _ := wrapText(string(rl.hint), GetTermWidth())

	offset += actual
	rl.hintY = offset - 1

	hintText := string(wrapped)

	if len(hintText) > 0 {
		print("\n")
		print("\r" + string(hintText) + seqReset)
	}
}

func (rl *Instance) resetHintText() {
	rl.hintY = 0
	rl.hint = []rune{}
	rl.histHint = []rune{}
}

// wrapText - Wraps a text given a specified width, and returns the formatted
// string as well the number of lines it will occupy.
func wrapText(text string, lineWidth int) (wrapped string, lines int) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}
	wrapped = words[0]
	spaceLeft := lineWidth - len(wrapped)
	// There must be at least a line
	if text != "" {
		lines++
	}
	for _, word := range words[1:] {
		if len(ansi.Strip(word))+1 > spaceLeft {
			lines++
			wrapped += "\n" + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}
	return
}
