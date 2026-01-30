package ast

// Position represents a position in source code.
type Position struct {
	Offset int // Byte offset from start of file
	Line   int // 1-indexed line number
	Column int // 1-indexed column number (in bytes)
}

// Range represents a span of source code.
type Range struct {
	Start Position
	End   Position
}

// NewPosition creates a new Position.
func NewPosition(offset, line, column int) Position {
	return Position{
		Offset: offset,
		Line:   line,
		Column: column,
	}
}

// NewRange creates a new Range from start and end positions.
func NewRange(start, end Position) Range {
	return Range{
		Start: start,
		End:   end,
	}
}

// IsValid returns true if the position has been set.
func (p Position) IsValid() bool {
	return p.Line > 0
}

// IsValid returns true if the range has been set.
func (r Range) IsValid() bool {
	return r.Start.IsValid()
}
