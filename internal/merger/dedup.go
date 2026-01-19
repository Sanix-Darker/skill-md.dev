package merger

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"github.com/sanixdarker/skillforge/pkg/skill"
)

const (
	// Number of hash functions for MinHash
	numHashFunctions = 100
	// Similarity threshold for deduplication
	similarityThreshold = 0.8
)

// Deduplicator handles content deduplication using MinHash/LSH.
type Deduplicator struct {
	hashSeeds []uint64
}

// NewDeduplicator creates a new Deduplicator.
func NewDeduplicator() *Deduplicator {
	seeds := make([]uint64, numHashFunctions)
	for i := range seeds {
		seeds[i] = uint64(i*31 + 7)
	}
	return &Deduplicator{hashSeeds: seeds}
}

// DeduplicateSections removes near-duplicate sections.
func (d *Deduplicator) DeduplicateSections(sections []skill.Section) []skill.Section {
	if len(sections) <= 1 {
		return sections
	}

	// Compute signatures for each section
	type sectionWithSig struct {
		section   skill.Section
		signature []uint64
		index     int
	}

	items := make([]sectionWithSig, len(sections))
	for i, sec := range sections {
		items[i] = sectionWithSig{
			section:   sec,
			signature: d.computeSignature(sec.Content),
			index:     i,
		}
	}

	// Find duplicates
	duplicates := make(map[int]bool)
	for i := 0; i < len(items); i++ {
		if duplicates[i] {
			continue
		}
		for j := i + 1; j < len(items); j++ {
			if duplicates[j] {
				continue
			}
			if d.similarity(items[i].signature, items[j].signature) >= similarityThreshold {
				// Keep the longer content
				if len(items[i].section.Content) >= len(items[j].section.Content) {
					duplicates[j] = true
				} else {
					duplicates[i] = true
					break
				}
			}
		}
	}

	// Filter out duplicates
	result := make([]skill.Section, 0, len(sections)-len(duplicates))
	for i, item := range items {
		if !duplicates[i] {
			result = append(result, item.section)
		}
	}

	return result
}

// computeSignature computes a MinHash signature for text.
func (d *Deduplicator) computeSignature(text string) []uint64 {
	// Generate shingles (word n-grams)
	shingles := d.generateShingles(text, 3)
	if len(shingles) == 0 {
		return make([]uint64, numHashFunctions)
	}

	// Compute MinHash signature
	signature := make([]uint64, numHashFunctions)
	for i := range signature {
		signature[i] = math.MaxUint64
	}

	for shingle := range shingles {
		h := fnv.New64a()
		h.Write([]byte(shingle))
		baseHash := h.Sum64()

		for i, seed := range d.hashSeeds {
			hash := baseHash ^ seed
			if hash < signature[i] {
				signature[i] = hash
			}
		}
	}

	return signature
}

// generateShingles creates word n-grams from text.
func (d *Deduplicator) generateShingles(text string, n int) map[string]bool {
	// Normalize text
	text = strings.ToLower(text)
	words := strings.Fields(text)

	shingles := make(map[string]bool)
	for i := 0; i <= len(words)-n; i++ {
		shingle := strings.Join(words[i:i+n], " ")
		shingles[shingle] = true
	}

	return shingles
}

// similarity computes Jaccard similarity between two signatures.
func (d *Deduplicator) similarity(sig1, sig2 []uint64) float64 {
	if len(sig1) != len(sig2) {
		return 0
	}

	matches := 0
	for i := range sig1 {
		if sig1[i] == sig2[i] {
			matches++
		}
	}

	return float64(matches) / float64(len(sig1))
}

// DeduplicateStrings removes near-duplicate strings from a slice.
func (d *Deduplicator) DeduplicateStrings(items []string) []string {
	if len(items) <= 1 {
		return items
	}

	type itemWithSig struct {
		text      string
		signature []uint64
		index     int
	}

	entries := make([]itemWithSig, len(items))
	for i, text := range items {
		entries[i] = itemWithSig{
			text:      text,
			signature: d.computeSignature(text),
			index:     i,
		}
	}

	duplicates := make(map[int]bool)
	for i := 0; i < len(entries); i++ {
		if duplicates[i] {
			continue
		}
		for j := i + 1; j < len(entries); j++ {
			if duplicates[j] {
				continue
			}
			if d.similarity(entries[i].signature, entries[j].signature) >= similarityThreshold {
				if len(entries[i].text) >= len(entries[j].text) {
					duplicates[j] = true
				} else {
					duplicates[i] = true
					break
				}
			}
		}
	}

	result := make([]string, 0, len(items)-len(duplicates))
	indices := make([]int, 0)
	for i := range entries {
		if !duplicates[i] {
			indices = append(indices, i)
		}
	}
	sort.Ints(indices)

	for _, i := range indices {
		result = append(result, entries[i].text)
	}

	return result
}
