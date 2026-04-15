package util

import (
	"bytes"
	"strings"
	"sync"
	"unicode"
)

var (
	// punctuationMap map of sentence end and pause punctuation marks
	punctuationMap = map[rune]bool{
		'。':  true,
		'？':  true,
		'！':  true,
		'；':  true,
		'：':  true,
		'\n': true,
		'.':  true,
		'?':  true,
		'!':  true,
		';':  true,
		':':  true,
	}

	// firstPunctuation punctuation map used for first time processing (includes comma)
	firstPunctuation = map[rune]bool{
		'，':  true,
		',':  true,
		'。':  true,
		'？':  true,
		'！':  true,
		'；':  true,
		'：':  true,
		'\n': true,
		'.':  true,
		'?':  true,
		'!':  true,
		';':  true,
		':':  true,
	}

	// sentence end punctuation marks
	sentenceEndPunctuation = []rune{'.', '。', '!', '！', '?', '？', '\n'}

	// sentence pause punctuation marks (can be used as break points for long sentences)
	sentencePausePunctuation = []rune{',', '，', ';', '；', ':', '：'}

	// object pool for reuse
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// slice pool for storing results
	runeSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]rune, 0, 1024)
			return &slice
		},
	}
)

// IsSentenceEndPunctuation judges whether a character is a sentence end punctuation mark
func IsSentenceEndPunctuation(r rune) bool {
	for _, p := range sentenceEndPunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// IsSentencePausePunctuation judges whether a character is a sentence pause punctuation mark
func IsSentencePausePunctuation(r rune) bool {
	for _, p := range sentencePausePunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// IsNumberWithDot judges whether a string is in number plus dot format (e.g. "1.", "2.", etc.)
func IsNumberWithDot(s string) bool {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 || trimmed[len(trimmed)-1] != '.' {
		return false
	}

	for i := 0; i < len(trimmed)-1; i++ {
		if !unicode.IsDigit(rune(trimmed[i])) {
			return false
		}
	}
	return true
}

// ExtractCompleteSentences extracts complete sentences from text
// Returns slice of complete sentences and remaining incomplete content
func ExtractCompleteSentences(text string) ([]string, string) {
	if text == "" {
		return []string{}, ""
	}

	var sentences []string
	var currentSentence bytes.Buffer

	runes := []rune(text)
	lastIndex := len(runes) - 1

	for i, r := range runes {
		currentSentence.WriteRune(r)

		// Judge whether sentence ends
		if IsSentenceEndPunctuation(r) {
			// If it's sentence end punctuation
			sentence := strings.TrimSpace(currentSentence.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			currentSentence.Reset()
		} else if i == lastIndex {
			// If it's the last character but not sentence end punctuation, keep in remaining
			break
		}
	}

	// Current incomplete sentence as remaining return
	remaining := currentSentence.String()
	return sentences, strings.TrimSpace(remaining)
}

// isNumberPrefix uses fast character inspection instead of regex to determine if it's a number prefix
func isNumberPrefix(text []rune, pos int) bool {
	if pos <= 0 || text[pos] != '.' {
		return false
	}

	// Find line start or newline character before
	start := pos - 1
	digitCount := 0
	foundDigit := false

	// Skip whitespace characters before dot
	for start >= 0 && (text[start] == ' ' || text[start] == '\t') {
		start--
	}

	// Count digits
	for start >= 0 && text[start] >= '0' && text[start] <= '9' {
		digitCount++
		foundDigit = true
		if digitCount > 3 { // More than 3 digits is not a valid number prefix
			return false
		}
		start--
	}

	// Check whether before digits is whitespace or line start
	if start >= 0 && text[start] != ' ' && text[start] != '\t' && text[start] != '\n' {
		return false
	}

	return foundDigit
}

// trimSpaceRunes removes leading and trailing whitespace characters
func trimSpaceRunes(text []rune) []rune {
	start, end := 0, len(text)-1

	for start <= end && (text[start] == ' ' || text[start] == '\t' || text[start] == '\n') {
		start++
	}

	for end >= start && (text[end] == ' ' || text[end] == '\t' || text[end] == '\n') {
		end--
	}

	if start > end {
		return nil
	}
	return text[start : end+1]
}

func isDigitAdjacentColon(text []rune, pos int) bool {
	if pos < 0 || pos >= len(text) {
		return false
	}

	colon := text[pos]
	if colon != ':' && colon != '：' {
		return false
	}

	if pos == 0 || !unicode.IsDigit(text[pos-1]) {
		return false
	}

	if pos == len(text)-1 {
		return true
	}

	return unicode.IsDigit(text[pos+1])
}

// findLastPunctuation finds the last punctuation from end to start
func findLastPunctuation(text []rune, separatorMap map[rune]bool) int {
	lastPos := -1
	for i := len(text) - 1; i >= 0; i-- {
		// Check if it's punctuation mark
		if separatorMap[text[i]] {
			// If it's dot, check if it's part of a number prefix
			if text[i] == '.' && isNumberPrefix(text, i) {
				continue
			}
			if isDigitAdjacentColon(text, i) {
				continue
			}
			return i
		}
	}
	return lastPos
}

// findNextSplitPoint finds the next split point
func findNextSplitPoint(text []rune, startPos int, maxLen int, separatorMap map[rune]bool) int {
	// Calculate end position of search
	endPos := startPos + maxLen
	if endPos > len(text) {
		endPos = len(text)
	}

	// Search from start to end
	for i := startPos; i < endPos; i++ {
		// Check if it's newline character, and at the same time check if next line is a numbered list
		if text[i] == '\n' {
			nextPos := i + 1
			// Skip whitespace characters
			for nextPos < endPos && (text[nextPos] == ' ' || text[nextPos] == '\t') {
				nextPos++
			}
			// Check if it's number prefix start
			if nextPos < endPos-2 && text[nextPos] >= '0' && text[nextPos] <= '9' {
				return i
			}
			continue
		}

		// Use map to check if it's punctuation mark
		if separatorMap[text[i]] {
			if isDigitAdjacentColon(text, i) {
				continue
			}
			return i
		}
	}

	// If not found within maxLen range, try to find in larger range
	if endPos < len(text) {
		for i := endPos; i < len(text); i++ {
			if text[i] == '\n' {
				return i
			}
			if separatorMap[text[i]] {
				if isDigitAdjacentColon(text, i) {
					continue
				}
				return i
			}
		}
	}

	return -1
}

// ExtractSmartSentences intelligently extracts sentences
// text: text to process
// minLen: minimum sentence length
// maxLen: maximum sentence length
// isFirst: whether it's first time processing (first time processing allows using comma as separator)
func ExtractSmartSentences(text string, minLen, maxLen int, isFirst bool) (sentences []string, remaining string) {
	// When isFirst is true, allow comma as separator
	separatorMap := punctuationMap
	if isFirst {
		separatorMap = firstPunctuation
	}
	// Pre-allocate reasonable slice capacity
	estimatedCount := len(text) / 50
	if estimatedCount < 10 {
		estimatedCount = 10
	}
	sentences = make([]string, 0, estimatedCount)

	// Convert to rune slice at once
	currentRunes := []rune(text)
	startPos := 0

	// Get reusable object from object pool
	builder := builderPool.Get().(*strings.Builder)
	defer builderPool.Put(builder)
	builder.Grow(maxLen * 2)

	// Get temporary rune slice
	tempRunesPtr := runeSlicePool.Get().(*[]rune)
	tempRunes := (*tempRunesPtr)[:0]
	defer runeSlicePool.Put(tempRunesPtr)

	for startPos < len(currentRunes) {
		// Skip leading whitespace characters
		for startPos < len(currentRunes) && (currentRunes[startPos] == ' ' || currentRunes[startPos] == '\t' || currentRunes[startPos] == '\n') {
			startPos++
		}

		if startPos >= len(currentRunes) {
			break
		}

		// Find next split point
		splitPos := findNextSplitPoint(currentRunes, startPos, maxLen, separatorMap)
		if splitPos == -1 {
			// No split point found, treat remaining text as remaining
			segment := trimSpaceRunes(currentRunes[startPos:])
			if len(segment) > 0 {
				remaining = string(segment)
			}
			break
		}

		// Extract current segment
		builder.Reset()
		tempRunes = tempRunes[:0]

		// Collect and process current segment
		segment := trimSpaceRunes(currentRunes[startPos : splitPos+1])

		// Check whether segment meets minimum length requirement and ends with punctuation
		if len(segment) >= minLen && separatorMap[segment[len(segment)-1]] {
			sentences = append(sentences, string(segment))
		} else {
			// If not meeting condition, add it to remaining
			if len(segment) > 0 {
				if len(remaining) > 0 {
					remaining += " "
				}
				remaining += string(segment)
			}
		}

		startPos = splitPos + 1
	}

	return sentences, remaining
}

// ContainsSentenceSeparator judges whether a string contains separators (sentence end or pause punctuation marks)
func ContainsSentenceSeparator(s string, isFirst bool) bool {
	separatorMap := punctuationMap
	if isFirst {
		separatorMap = firstPunctuation
	}

	runes := []rune(s)
	for i, r := range runes {
		if !separatorMap[r] {
			continue
		}
		if isDigitAdjacentColon(runes, i) {
			continue
		}
		return true
	}

	return false
}
