package readline

import (
	"strconv"
	"strings"
)

//
// Iterations ------------------------------------------------------------------ //
//

func (rl *Instance) addIteration(i string) {
	// Either reset
	if i == "" {
		rl.iterations = ""
	}

	// Add a negative argument
	if rl.negativeArg {
		rl.iterations = ""
		i = "-" + i
	}

	// Or add the negative or the positive.
	rl.iterations += i
	rl.negativeArg = false
}

func (rl *Instance) getIterations() int {
	i, err := strconv.Atoi(rl.iterations)

	// Emacs accepts negative args
	if rl.main != emacs && i <= 0 ||
		rl.main == emacs && err != nil {
		i = 1
	}

	rl.iterations = ""
	return i
}

//
// Movement -------------------------------------------------------------------- //
//

func (rl *Instance) viJumpB(tokeniser tokeniser) (adjust int) {
	split, index, pos := tokeniser(rl.line, rl.pos)
	switch {
	case len(split) == 0:
		return
	case index == 0 && pos == 0:
		return
	case pos == 0:
		adjust = len(split[index-1])
	default:
		adjust = pos
	}
	return adjust * -1
}

func (rl *Instance) viJumpE(tokeniser tokeniser) (adjust int) {
	split, index, pos := tokeniser(rl.line, rl.pos)
	if len(split) == 0 {
		return
	}

	word := rTrimWhiteSpace(split[index])

	switch {
	case len(split) == 0:
		return
	case index == len(split)-1 && pos >= len(word)-1:
		return
	case pos >= len(word)-1:
		word = rTrimWhiteSpace(split[index+1])
		adjust = len(split[index]) - pos
		adjust += len(word) - 1
	default:
		adjust = len(word) - pos - 1
	}
	return
}

func (rl *Instance) viJumpW(tokeniser tokeniser) (adjust int) {
	split, index, pos := tokeniser(rl.line, rl.pos)
	switch {
	case len(split) == 0:
		return
	case index+1 == len(split):
		adjust = len(rl.line) - rl.pos
	default:
		adjust = len(split[index]) - pos
	}
	return
}

func (rl *Instance) viJumpPreviousBrace() (adjust int) {
	if rl.pos == 0 {
		return 0
	}

	for i := rl.pos - 1; i != 0; i-- {
		if rl.line[i] == '{' {
			return i - rl.pos
		}
	}

	return 0
}

func (rl *Instance) viJumpNextBrace() (adjust int) {
	if rl.pos >= len(rl.line)-1 {
		return 0
	}

	for i := rl.pos + 1; i < len(rl.line); i++ {
		if rl.line[i] == '{' {
			return i - rl.pos
		}
	}

	return 0
}

func (rl *Instance) viJumpBracket() (adjust int) {
	split, index, pos := tokeniseBrackets(rl.line, rl.pos)
	switch {
	case len(split) == 0:
		return
	case pos == 0:
		adjust = len(split[index])
	default:
		adjust = pos * -1
	}
	return
}

//
// Matchers -------------------------------------------------------------------- //
//

func (rl *Instance) matchSurround(r rune) (bchar, echar rune) {
	bchar = r
	echar = r

	switch bchar {
	case '{':
		echar = '}'
	case '(':
		echar = ')'
	case '[':
		echar = ']'
	case '<':
		echar = '>'
	case '}':
		bchar = '{'
		echar = '}'
	case ')':
		bchar = '('
		echar = ')'
	case ']':
		bchar = '['
		echar = ']'
	case '>':
		bchar = '<'
		echar = '>'
	}

	return
}

func isBracket(r rune) bool {
	if r == '(' ||
		r == ')' ||
		r == '{' ||
		r == '}' ||
		r == '[' ||
		r == ']' {
		return true
	}

	return false
}

func (rl *Instance) matches(r, e rune) bool {
	switch r {
	case '{':
		return e == '}'
	case '(':
		return e == ')'
	case '[':
		return e == ']'
	case '<':
		return e == '>'
	case '"':
		return e == '"'
	case '\'':
		return e == '\''
	}

	return e == r
}

//
// Deletion -------------------------------------------------------------------- //
//

func (rl *Instance) viDeleteByAdjust(adjust int) {
	var newLine []rune

	// Avoid doing anything if input line is empty.
	if len(rl.line) == 0 {
		return
	}

	switch {
	case adjust == 0:
		rl.skipUndoAppend()
		return
	case rl.pos+adjust >= len(rl.line)-1:
		newLine = rl.line[:rl.pos-1]
	case rl.pos+adjust == 0:
		newLine = rl.line[rl.pos:]
	case adjust < 0:
		newLine = append(rl.line[:rl.pos+adjust], rl.line[rl.pos:]...)
	default:
		newLine = append(rl.line[:rl.pos], rl.line[rl.pos+adjust:]...)
	}

	rl.line = newLine

	if adjust < 0 {
		rl.moveCursorByAdjust(adjust)
	}
}

func (rl *Instance) vimDeleteToken(r rune) bool {
	tokens, _, _ := tokeniseSplitSpaces(rl.line, 0)
	pos := int(r) - 48 // convert ASCII to integer
	if pos > len(tokens) {
		return false
	}

	s := string(rl.line)
	newLine := strings.ReplaceAll(s, tokens[pos-1], "")
	if newLine == s {
		return false
	}

	rl.line = []rune(newLine)

	rl.redisplay()

	if rl.pos > len(rl.line) {
		rl.pos = len(rl.line) - 1
	}

	return true
}
