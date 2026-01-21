package merger

import (
	"strings"
	"testing"

	"github.com/sanixdarker/skill-md/pkg/skill"
)

func TestDeduplicator_Empty(t *testing.T) {
	d := NewDeduplicator()

	t.Run("nil sections", func(t *testing.T) {
		result := d.DeduplicateSections(nil)
		if result != nil {
			t.Errorf("expected nil for nil input, got %v", result)
		}
	})

	t.Run("empty sections", func(t *testing.T) {
		result := d.DeduplicateSections([]skill.Section{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d sections", len(result))
		}
	})

	t.Run("single section", func(t *testing.T) {
		input := []skill.Section{{Title: "Test", Content: "Content"}}
		result := d.DeduplicateSections(input)
		if len(result) != 1 {
			t.Errorf("expected 1 section, got %d", len(result))
		}
	})
}

func TestDeduplicator_SimilarityThreshold(t *testing.T) {
	d := NewDeduplicator()

	// Create two sections with 80%+ similar content
	content1 := "This is a test section with some content about API endpoints and their usage patterns."
	content2 := "This is a test section with some content about API endpoints and their usage patterns."

	sections := []skill.Section{
		{Title: "Test 1", Content: content1},
		{Title: "Test 2", Content: content2},
	}

	result := d.DeduplicateSections(sections)

	// Identical content should result in only one section
	if len(result) != 1 {
		t.Errorf("expected 1 section after deduplication of identical content, got %d", len(result))
	}
}

func TestDeduplicator_KeepLonger(t *testing.T) {
	d := NewDeduplicator()

	// Two similar sections, one longer
	shortContent := "This is a test API overview with endpoints."
	longContent := "This is a test API overview with endpoints. It includes additional details about authentication, rate limiting, and error handling."

	sections := []skill.Section{
		{Title: "Short", Content: shortContent},
		{Title: "Long", Content: longContent},
	}

	result := d.DeduplicateSections(sections)

	// Should keep the longer content when similar
	if len(result) == 1 {
		if !strings.Contains(result[0].Content, "additional details") {
			t.Error("expected to keep the longer content")
		}
	}
}

func TestDeduplicator_computeSignature_EmptyText(t *testing.T) {
	d := NewDeduplicator()

	signature := d.computeSignature("")

	// Should return a signature of the correct length even for empty text
	if len(signature) != numHashFunctions {
		t.Errorf("expected signature length %d, got %d", numHashFunctions, len(signature))
	}
}

func TestDeduplicator_computeSignature_Consistent(t *testing.T) {
	d := NewDeduplicator()

	text := "This is a test string for signature computation."

	sig1 := d.computeSignature(text)
	sig2 := d.computeSignature(text)

	// Same text should produce same signature
	for i := range sig1 {
		if sig1[i] != sig2[i] {
			t.Errorf("inconsistent signature at index %d: %d != %d", i, sig1[i], sig2[i])
		}
	}
}

func TestDeduplicator_similarity(t *testing.T) {
	d := NewDeduplicator()

	tests := []struct {
		name     string
		sig1     []uint64
		sig2     []uint64
		expected float64
	}{
		{
			name:     "identical signatures",
			sig1:     []uint64{1, 2, 3, 4, 5},
			sig2:     []uint64{1, 2, 3, 4, 5},
			expected: 1.0,
		},
		{
			name:     "completely different",
			sig1:     []uint64{1, 2, 3, 4, 5},
			sig2:     []uint64{6, 7, 8, 9, 10},
			expected: 0.0,
		},
		{
			name:     "50% similar",
			sig1:     []uint64{1, 2, 3, 4},
			sig2:     []uint64{1, 2, 8, 9},
			expected: 0.5,
		},
		{
			name:     "different lengths",
			sig1:     []uint64{1, 2, 3},
			sig2:     []uint64{1, 2, 3, 4, 5},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.similarity(tt.sig1, tt.sig2)
			if result != tt.expected {
				t.Errorf("expected similarity %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestDeduplicator_generateShingles(t *testing.T) {
	d := NewDeduplicator()

	tests := []struct {
		name     string
		text     string
		n        int
		expected int // minimum expected shingles
	}{
		{
			name:     "empty text",
			text:     "",
			n:        3,
			expected: 0,
		},
		{
			name:     "text shorter than n",
			text:     "one two",
			n:        3,
			expected: 0,
		},
		{
			name:     "exact n words",
			text:     "one two three",
			n:        3,
			expected: 1,
		},
		{
			name:     "more than n words",
			text:     "one two three four five",
			n:        3,
			expected: 3, // "one two three", "two three four", "three four five"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shingles := d.generateShingles(tt.text, tt.n)
			if len(shingles) != tt.expected {
				t.Errorf("expected %d shingles, got %d", tt.expected, len(shingles))
			}
		})
	}
}

func TestDeduplicator_generateShingles_CaseInsensitive(t *testing.T) {
	d := NewDeduplicator()

	upper := d.generateShingles("ONE TWO THREE", 3)
	lower := d.generateShingles("one two three", 3)

	// Should produce the same shingles regardless of case
	if len(upper) != len(lower) {
		t.Errorf("case sensitivity: different number of shingles")
	}

	for shingle := range upper {
		if !lower[shingle] {
			t.Errorf("shingle %q not found in lowercase version", shingle)
		}
	}
}

func TestDeduplicator_DeduplicateStrings(t *testing.T) {
	d := NewDeduplicator()

	t.Run("empty input", func(t *testing.T) {
		result := d.DeduplicateStrings([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d items", len(result))
		}
	})

	t.Run("single item", func(t *testing.T) {
		result := d.DeduplicateStrings([]string{"test"})
		if len(result) != 1 {
			t.Errorf("expected 1 item, got %d", len(result))
		}
	})

	t.Run("duplicate strings", func(t *testing.T) {
		input := []string{
			"This is a test string with some content about APIs.",
			"This is a test string with some content about APIs.",
		}
		result := d.DeduplicateStrings(input)
		if len(result) != 1 {
			t.Errorf("expected 1 item after dedup, got %d", len(result))
		}
	})

	t.Run("distinct strings", func(t *testing.T) {
		input := []string{
			"First completely unique string about users.",
			"Second completely different string about products.",
		}
		result := d.DeduplicateStrings(input)
		if len(result) != 2 {
			t.Errorf("expected 2 distinct items, got %d", len(result))
		}
	})
}

func TestDeduplicator_DeduplicateStrings_PreservesOrder(t *testing.T) {
	d := NewDeduplicator()

	input := []string{
		"First string with unique content.",
		"Second string with different content.",
		"Third string with more content here.",
	}

	result := d.DeduplicateStrings(input)

	// Order should be preserved
	for i, item := range result {
		if item != input[i] {
			t.Errorf("order not preserved at index %d: expected %q, got %q", i, input[i], item)
		}
	}
}

func TestDeduplicator_DeduplicateSections_KeepsLongerContent(t *testing.T) {
	d := NewDeduplicator()

	// Two very similar sections where second is longer
	sections := []skill.Section{
		{
			Title:   "API Overview",
			Content: "This API provides user management features.",
		},
		{
			Title:   "API Overview Extended",
			Content: "This API provides user management features. It also includes authentication, rate limiting, and comprehensive error handling capabilities.",
		},
	}

	result := d.DeduplicateSections(sections)

	// The longer content should be kept
	if len(result) == 1 {
		if !strings.Contains(result[0].Content, "comprehensive error handling") {
			t.Error("expected to keep the longer, more detailed content")
		}
	}
}

func TestDeduplicator_DeduplicateSections_MultipleDuplicates(t *testing.T) {
	d := NewDeduplicator()

	// Multiple duplicate pairs
	sections := []skill.Section{
		{Title: "A", Content: "User API endpoint for managing users in the system."},
		{Title: "B", Content: "User API endpoint for managing users in the system."},
		{Title: "C", Content: "Product API endpoint for managing products in the catalog."},
		{Title: "D", Content: "Product API endpoint for managing products in the catalog."},
	}

	result := d.DeduplicateSections(sections)

	// Should have 2 sections after removing duplicates
	if len(result) != 2 {
		t.Errorf("expected 2 sections, got %d", len(result))
	}
}

func TestNewDeduplicator(t *testing.T) {
	d := NewDeduplicator()

	if d == nil {
		t.Fatal("NewDeduplicator returned nil")
	}

	if len(d.hashSeeds) != numHashFunctions {
		t.Errorf("expected %d hash seeds, got %d", numHashFunctions, len(d.hashSeeds))
	}

	// Verify seeds are unique
	seen := make(map[uint64]bool)
	for i, seed := range d.hashSeeds {
		if seen[seed] {
			t.Errorf("duplicate seed at index %d", i)
		}
		seen[seed] = true
	}
}
