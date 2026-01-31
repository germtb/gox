package generator

import (
	"testing"
)

func TestSourceMapBasicMapping(t *testing.T) {
	sm := NewSourceMap()

	// Add a simple mapping
	sm.AddMapping(10, 5, 20, 10) // .gox line 10, col 5 -> .go line 20, col 10

	// Test forward lookup (source to target)
	pos, ok := sm.TargetPositionFromSource(10, 5)
	if !ok {
		t.Fatal("Expected to find target position")
	}
	if pos.Line != 20 || pos.Column != 10 {
		t.Errorf("Expected line 20, col 10, got line %d, col %d", pos.Line, pos.Column)
	}

	// Test reverse lookup (target to source)
	pos, ok = sm.SourcePositionFromTarget(20, 10)
	if !ok {
		t.Fatal("Expected to find source position")
	}
	if pos.Line != 10 || pos.Column != 5 {
		t.Errorf("Expected line 10, col 5, got line %d, col %d", pos.Line, pos.Column)
	}
}

func TestSourceMapExpressionMapping(t *testing.T) {
	sm := NewSourceMap()

	// Map an expression "hello" starting at source (5, 10) and target (15, 20)
	srcStart := NewPosition(0, 5, 10)
	tgtStart := NewPosition(0, 15, 20)
	sm.AddExpression("hello", srcStart, tgtStart)

	// Check each character is mapped
	// h at col 10, e at col 11, l at col 12, l at col 13, o at col 14
	for i := uint32(0); i <= 5; i++ { // 5 chars + 1 for end position
		pos, ok := sm.TargetPositionFromSource(5, 10+i)
		if !ok {
			t.Fatalf("Expected to find mapping for col %d", 10+i)
		}
		if pos.Line != 15 || pos.Column != 20+i {
			t.Errorf("Col %d: expected target col %d, got %d", 10+i, 20+i, pos.Column)
		}
	}
}

func TestSourceMapMultilineExpression(t *testing.T) {
	sm := NewSourceMap()

	// Map a multiline expression
	srcStart := NewPosition(0, 10, 5)
	tgtStart := NewPosition(0, 20, 5)
	sm.AddExpression("line1\nline2", srcStart, tgtStart)

	// First line mappings
	pos, ok := sm.TargetPositionFromSource(10, 5)
	if !ok {
		t.Fatal("Expected to find first line mapping")
	}
	if pos.Line != 20 {
		t.Errorf("Expected target line 20, got %d", pos.Line)
	}

	// Second line mappings (line increments, col resets to 0)
	pos, ok = sm.TargetPositionFromSource(11, 0)
	if !ok {
		t.Fatal("Expected to find second line mapping")
	}
	if pos.Line != 21 || pos.Column != 0 {
		t.Errorf("Expected target line 21, col 0, got line %d, col %d", pos.Line, pos.Column)
	}
}

func TestSourceMapBackwardSearch(t *testing.T) {
	sm := NewSourceMap()

	// Add sparse mappings on a line
	sm.AddMapping(10, 0, 20, 0)
	sm.AddMapping(10, 10, 20, 10)

	// Query for a column that doesn't have exact mapping
	pos, ok := sm.SourcePositionFromTarget(20, 5) // Between 0 and 10
	if !ok {
		t.Fatal("Expected to find mapping via backward search")
	}
	// Should find the closest mapping at or before col 5, which is col 0
	if pos.Column != 0 {
		t.Errorf("Expected source col 0, got %d", pos.Column)
	}

	// Query for col 12, should find col 10
	pos, ok = sm.SourcePositionFromTarget(20, 12)
	if !ok {
		t.Fatal("Expected to find mapping via backward search")
	}
	if pos.Column != 10 {
		t.Errorf("Expected source col 10, got %d", pos.Column)
	}
}

func TestSourceMapJSON(t *testing.T) {
	sm := NewSourceMap()
	sm.SetFiles("test.gox", "test_gox.go")
	sm.AddMapping(1, 1, 10, 10)

	// Serialize
	data, err := sm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	// Deserialize
	sm2, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}

	if sm2.SourceFile != "test.gox" {
		t.Errorf("Expected source file 'test.gox', got '%s'", sm2.SourceFile)
	}
	if sm2.TargetFile != "test_gox.go" {
		t.Errorf("Expected target file 'test_gox.go', got '%s'", sm2.TargetFile)
	}

	pos, ok := sm2.TargetPositionFromSource(1, 1)
	if !ok {
		t.Fatal("Expected to find mapping after deserialization")
	}
	if pos.Line != 10 || pos.Column != 10 {
		t.Errorf("Expected line 10, col 10, got line %d, col %d", pos.Line, pos.Column)
	}
}

func TestSourceMapHasMappings(t *testing.T) {
	sm := NewSourceMap()

	if sm.HasMappings() {
		t.Error("Empty source map should not have mappings")
	}

	sm.AddMapping(1, 1, 1, 1)

	if !sm.HasMappings() {
		t.Error("Source map with mapping should have mappings")
	}
}
