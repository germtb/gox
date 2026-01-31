package generator

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Position represents a location in a file.
type Position struct {
	Index  int64  `json:"index"`  // Byte offset
	Line   uint32 `json:"line"`   // 0-indexed line number
	Column uint32 `json:"column"` // 0-indexed column (in runes, not bytes)
}

// NewPosition creates a new Position.
func NewPosition(index int64, line, col uint32) Position {
	return Position{Index: index, Line: line, Column: col}
}

// Range represents a range in a file.
type Range struct {
	From Position `json:"from"`
	To   Position `json:"to"`
}

// SourceMap provides bidirectional mapping between source (.gox) and target (.go) positions.
// Uses nested maps for O(1) lookups: map[line]map[column]Position
type SourceMap struct {
	// SourceFile is the original .gox file path
	SourceFile string `json:"sourceFile"`

	// TargetFile is the generated .go file path
	TargetFile string `json:"targetFile"`

	// SourceToTarget maps .gox positions to .go positions
	SourceToTarget map[uint32]map[uint32]Position `json:"sourceToTarget"`

	// TargetToSource maps .go positions to .gox positions
	TargetToSource map[uint32]map[uint32]Position `json:"targetToSource"`
}

// NewSourceMap creates a new SourceMap.
func NewSourceMap() *SourceMap {
	return &SourceMap{
		SourceToTarget: make(map[uint32]map[uint32]Position),
		TargetToSource: make(map[uint32]map[uint32]Position),
	}
}

// SetFiles sets the source and target file paths.
func (sm *SourceMap) SetFiles(source, target string) {
	sm.SourceFile = source
	sm.TargetFile = target
}

// AddMapping adds a character-level mapping between source and target positions.
func (sm *SourceMap) AddMapping(srcLine, srcCol uint32, tgtLine, tgtCol uint32) {
	// Source to target
	if _, ok := sm.SourceToTarget[srcLine]; !ok {
		sm.SourceToTarget[srcLine] = make(map[uint32]Position)
	}
	sm.SourceToTarget[srcLine][srcCol] = NewPosition(0, tgtLine, tgtCol)

	// Target to source
	if _, ok := sm.TargetToSource[tgtLine]; !ok {
		sm.TargetToSource[tgtLine] = make(map[uint32]Position)
	}
	sm.TargetToSource[tgtLine][tgtCol] = NewPosition(0, srcLine, srcCol)
}

// AddExpression adds character-by-character mappings for an expression.
// srcStart is the position in the source file, tgtStart is the position in the target file.
// The expression value is used to calculate the mapping for each character.
func (sm *SourceMap) AddExpression(value string, srcStart, tgtStart Position) {
	lines := strings.Split(value, "\n")

	var srcLine, tgtLine uint32 = srcStart.Line, tgtStart.Line
	var srcCol, tgtCol uint32 = srcStart.Column, tgtStart.Column

	for lineIndex, line := range lines {
		if lineIndex > 0 {
			// After first line, columns reset to 0
			srcLine++
			tgtLine++
			srcCol = 0
			tgtCol = 0
		}

		// Map each character in the line
		for _, r := range line {
			sm.AddMapping(srcLine, srcCol, tgtLine, tgtCol)

			// Move forward by rune width
			rlen := utf8.RuneLen(r)
			if rlen < 0 {
				rlen = 1
			}
			srcCol += uint32(rlen)
			tgtCol += uint32(rlen)
		}

		// Include newline character position
		sm.AddMapping(srcLine, srcCol, tgtLine, tgtCol)
	}
}

// TargetPositionFromSource looks up the target (.go) position from a source (.gox) position.
// Returns the exact mapping if found, otherwise returns false.
func (sm *SourceMap) TargetPositionFromSource(line, col uint32) (Position, bool) {
	lineMap, ok := sm.SourceToTarget[line]
	if !ok {
		return Position{}, false
	}

	// Try exact match first
	if pos, ok := lineMap[col]; ok {
		return pos, true
	}

	// Search backward on same line for closest mapping (within 5 columns)
	for c := col; c > 0 && col-c < 5; c-- {
		if pos, ok := lineMap[c]; ok {
			// Adjust column offset
			offset := col - c
			return Position{Line: pos.Line, Column: pos.Column + offset}, true
		}
	}

	return Position{}, false
}

// SourcePositionFromTarget looks up the source (.gox) position from a target (.go) position.
// If exact column not found, searches backward on the same line, then previous lines.
func (sm *SourceMap) SourcePositionFromTarget(line, col uint32) (Position, bool) {
	// First try the current line
	if lineMap, ok := sm.TargetToSource[line]; ok {
		// Try exact match first
		if pos, ok := lineMap[col]; ok {
			return pos, true
		}

		// Search backward for closest mapping on this line
		for c := col; c > 0; c-- {
			if pos, ok := lineMap[c]; ok {
				return pos, true
			}
		}

		// Try column 0 of current line
		if pos, ok := lineMap[0]; ok {
			return pos, true
		}
	}

	// Search previous lines for any mapping
	for l := line; l > 0; l-- {
		prevLine := l - 1
		if lineMap, ok := sm.TargetToSource[prevLine]; ok {
			// Find the highest column on this line
			var bestCol uint32 = 0
			var bestPos Position
			found := false
			for c, pos := range lineMap {
				if c >= bestCol {
					bestCol = c
					bestPos = pos
					found = true
				}
			}
			if found {
				return bestPos, true
			}
		}
	}

	return Position{}, false
}

// ToJSON serializes the source map to JSON.
func (sm *SourceMap) ToJSON() ([]byte, error) {
	return json.MarshalIndent(sm, "", "  ")
}

// FromJSON deserializes a source map from JSON.
func FromJSON(data []byte) (*SourceMap, error) {
	sm := &SourceMap{}
	if err := json.Unmarshal(data, sm); err != nil {
		return nil, err
	}
	return sm, nil
}

// HasMappings returns true if the source map contains any mappings.
func (sm *SourceMap) HasMappings() bool {
	return len(sm.SourceToTarget) > 0 || len(sm.TargetToSource) > 0
}

// FindTargetLine finds the target line for a given source line.
// Returns the first target line found for any column on the source line.
func (sm *SourceMap) FindTargetLine(srcLine uint32) (uint32, bool) {
	lineMap, ok := sm.SourceToTarget[srcLine]
	if !ok || len(lineMap) == 0 {
		return 0, false
	}

	// Return the target line from any mapping on this line
	for _, pos := range lineMap {
		return pos.Line, true
	}
	return 0, false
}

// FindSourceLine finds the source line for a given target line.
// Returns the first source line found for any column on the target line.
func (sm *SourceMap) FindSourceLine(tgtLine uint32) (uint32, bool) {
	lineMap, ok := sm.TargetToSource[tgtLine]
	if !ok || len(lineMap) == 0 {
		return 0, false
	}

	// Return the source line from any mapping on this line
	for _, pos := range lineMap {
		return pos.Line, true
	}
	return 0, false
}
