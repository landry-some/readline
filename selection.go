package readline

import (
	"regexp"
)

//
// Main selection helpers ------------------------------------------------------ //
//

// markSelection starts an active region at the specified mark position.
func (rl *Instance) markSelection(mark int) {
	rl.mark = mark
	rl.activeRegion = true
}

// Compute begin and end of region
func (rl *Instance) getSelectionPos() (bpos, epos, cpos int) {
	if rl.mark < rl.pos {
		bpos = rl.mark
		epos = rl.pos + 1
	} else {
		bpos = rl.pos
		epos = rl.mark
	}

	// Here, compute for visual line mode if needed.
	// We select the whole line.
	if rl.visualLine {
		bpos = 0

		for i, char := range rl.line[epos:] {
			if string(char) == "\n" {
				break
			}
			epos = epos + i
		}
	}

	// Ensure nothing is out of bounds
	if epos > len(rl.line)-1 {
		epos = len(rl.line)
	}
	if bpos < 0 {
		bpos = 0
	}

	cpos = bpos

	return
}

// popSelection returns the active region and resets it.
func (rl *Instance) popSelection() (s string, bpos, epos, cpos int) {
	bpos, epos, cpos = rl.getSelectionPos()
	s = string(rl.line[bpos:epos])

	rl.resetSelection()

	return
}

// insertBlock inserts a given string at the specified indexes, with
// an optional string to surround the block with. Everything before
// bpos and after epos is retained from the line, word inserted in the middle.
func (rl *Instance) insertBlock(bpos, epos int, word, surround string) {
	if surround != "" {
		word = surround + word + surround
	}

	begin := string(rl.line[:bpos])
	end := string(rl.line[epos:])

	newLine := append([]rune(begin), []rune(word)...)
	newLine = append(newLine, []rune(end)...)
	rl.line = newLine
}

// insertSelection works the same as insertBlock, but directly uses the
// current active region as a block to insert. Resets the selection once done.
// Returns the computed cursor position after insert.
func (rl *Instance) insertSelection(surround string) (wlen, cpos int) {
	bpos, epos, cpos := rl.getSelectionPos()
	selection := string(rl.line[bpos:epos])

	if surround != "" {
		selection = surround + selection + surround
	}

	begin := string(rl.line[:bpos])
	end := string(rl.line[epos:])

	newLine := append([]rune(begin), []rune(selection)...)
	newLine = append(newLine, []rune(end)...)
	rl.line = newLine

	rl.resetSelection()

	return len(selection), cpos
}

// yankSelection copies the active selection in the active/default register.
func (rl *Instance) yankSelection() {
	// Get the selection.
	bpos, epos, cpos := rl.getSelectionPos()
	selection := string(rl.line[bpos:epos])

	// The visual line mode always adds a newline
	if rl.local == visual && rl.visualLine {
		selection += "\n"
	}

	// And copy to active register
	rl.saveBufToRegister([]rune(selection))

	// and reset the cursor position if not in visual mode
	if !rl.visualLine {
		rl.pos = cpos
	}
}

// yankSelection deletes the active selection.
func (rl *Instance) deleteSelection() {
	var newline []rune

	// Get the selection.
	bpos, epos, cpos := rl.getSelectionPos()
	selection := string(rl.line[bpos:epos])

	// Save it and update the line
	rl.saveBufToRegister([]rune(selection))
	newline = append(rl.line[:bpos], rl.line[epos:]...)
	rl.line = newline

	rl.pos = cpos

	// Reset the selection since it does not exist anymore.
	rl.resetSelection()
}

// resetSelection unmarks the mark position and deactivates the region.
func (rl *Instance) resetSelection() {
	rl.activeRegion = false
	rl.mark = -1
}

//
// Selection search/modification helpers ----------------------------------------- //
//

// selectInWord returns the entire non-blank word around specified cursor position.
func (rl *Instance) selectInWord(cpos int) (bpos, epos int) {
	pattern := "[0-9a-zA-Z_]"
	bpos, epos = cpos, cpos

	if match, _ := regexp.MatchString(pattern, string(rl.line[cpos])); !match {
		pattern = "[^0-9a-zA-Z_ ]"
	}

	// To first space found backward
	for ; bpos >= 0; bpos-- {
		if match, _ := regexp.MatchString(pattern, string(rl.line[bpos])); !match {
			break
		}
	}

	// And to first space found forward
	for ; epos < len(rl.line); epos++ {
		if match, _ := regexp.MatchString(pattern, string(rl.line[epos])); !match {
			break
		}
	}

	bpos++

	// Ending position must be greater than 0
	if epos > 0 {
		epos--
	}

	return
}

// searchSurround returns the index of the enclosing rune (either matching signs
// or the rune itself) of the input line, as well as each enclosing char.
func (rl *Instance) searchSurround(r rune) (bpos, epos int, bchar, echar rune) {
	posInit := rl.pos

	bchar, echar = rl.matchSurround(r)

	bpos = rl.substrPos(bchar, false)
	epos = rl.substrPos(echar, true)

	if bpos == epos {
		rl.pos++
		epos = rl.substrPos(echar, true)
		if epos == -1 {
			rl.pos--
			epos = rl.substrPos(echar, false)
			if epos != -1 {
				tmp := epos
				epos = bpos
				bpos = tmp
			}
		}
	}

	rl.pos = posInit

	return
}

// adjustSurroundQuotes returns the correct mark and cursor positions when
// we want to know where a shell word enclosed with quotes (and potentially
// having inner ones) starts and ends.
func adjustSurroundQuotes(dBpos, dEpos, sBpos, sEpos int) (mark, cpos int) {
	mark = -1
	cpos = -1

	if (sBpos == -1 || sEpos == -1) && (dBpos == -1 || dEpos == -1) {
		return
	}

	doubleFirstAndValid := (dBpos < sBpos && // Outtermost
		dBpos >= 0 && // Double found
		sBpos >= 0 && // compared with a found single
		dEpos > sEpos) // ensuring that we are not comparing unfound

	singleFirstAndValid := (sBpos < dBpos &&
		sBpos >= 0 &&
		dBpos >= 0 &&
		sEpos > dEpos)

	if (sBpos == -1 || sEpos == -1) || doubleFirstAndValid {
		mark = dBpos
		cpos = dEpos
	} else if (dBpos == -1 || dEpos == -1) || singleFirstAndValid {
		mark = sBpos
		cpos = sEpos
	}

	return
}
