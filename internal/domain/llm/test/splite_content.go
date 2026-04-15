package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	// define punctuation set
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

	// used for reusable object pool
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// used for storing result slice pool
	runeSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]rune, 0, 1024)
			return &slice
		},
	}

	// pre-compile regex
	numberPrefixRegex = regexp.MustCompile(`(?m)^[\s]*\d{1,3}\.$`)
)

// use fast char inspection instead of regex
func isNumberPrefix(text []rune, pos int) bool {
	if pos <= 0 || text[pos] != '.' {
		return false
	}

	// find line start or newline char before
	start := pos - 1
	digitCount := 0
	foundDigit := false

	// skip whitespace before dot
	for start >= 0 && (text[start] == ' ' || text[start] == '\t') {
		start--
	}

	// count digits
	for start >= 0 && text[start] >= '0' && text[start] <= '9' {
		digitCount++
		foundDigit = true
		if digitCount > 3 { // more than 3 digits is not valid number
			return false
		}
		start--
	}

	// check if whitespace or line start before digits
	if start >= 0 && text[start] != ' ' && text[start] != '\t' && text[start] != '\n' {
		return false
	}

	return foundDigit
}

// trim leading and trailing whitespace
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

func findLastPunctuation(text []rune) int {
	// find last punctuation from end to start
	lastPos := -1
	for i := len(text) - 1; i >= 0; i-- {
		// check if punctuation
		if punctuationMap[text[i]] {
			// if dot, check if part of number
			if text[i] == '.' && isNumberPrefix(text, i) {
				continue
			}
			return i
		}
	}
	return lastPos
}

func findNextSplitPoint(text []rune, startPos int, maxLen int) int {
	// calculate end position
	endPos := startPos + maxLen
	if endPos > len(text) {
		endPos = len(text)
	}

	// find from start to end
	for i := startPos; i < endPos; i++ {
		// check if newline, also check if next line is number
		if text[i] == '\n' {
			nextPos := i + 1
			// skip whitespace
			for nextPos < endPos && (text[nextPos] == ' ' || text[nextPos] == '\t') {
				nextPos++
			}
			// check if number start
			if nextPos < endPos-2 && text[nextPos] >= '0' && text[nextPos] <= '9' {
				return i
			}
			continue
		}

		// use map to check if punctuation
		if punctuationMap[text[i]] {
			return i
		}
	}

	// if not found in maxLen range, try larger range
	if endPos < len(text) {
		for i := endPos; i < len(text); i++ {
			if text[i] == '\n' || punctuationMap[text[i]] {
				return i
			}
		}
	}

	return -1
}

func extractSmartSentences(text string, minLen, maxLen int) (sentences []string, remaining string) {
	// pre-allocate reasonable slice capacity
	estimatedCount := len(text) / 50
	if estimatedCount < 10 {
		estimatedCount = 10
	}
	sentences = make([]string, 0, estimatedCount)

	// convert to rune slice at once
	currentRunes := []rune(text)
	startPos := 0

	// get reusable object from pool
	builder := builderPool.Get().(*strings.Builder)
	defer builderPool.Put(builder)
	builder.Grow(maxLen * 2)

	// get temporary rune slice
	tempRunesPtr := runeSlicePool.Get().(*[]rune)
	tempRunes := (*tempRunesPtr)[:0]
	defer runeSlicePool.Put(tempRunesPtr)

	for startPos < len(currentRunes) {
		// skip leading whitespace
		for startPos < len(currentRunes) && (currentRunes[startPos] == ' ' || currentRunes[startPos] == '\t' || currentRunes[startPos] == '\n') {
			startPos++
		}

		if startPos >= len(currentRunes) {
			break
		}

		// find next split point
		splitPos := findNextSplitPoint(currentRunes, startPos, maxLen)
		if splitPos == -1 {
			// no split point found, treat remaining text as remaining
			segment := trimSpaceRunes(currentRunes[startPos:])
			if len(segment) > 0 {
				remaining = string(segment)
			}
			break
		}

		// extract current segment
		builder.Reset()
		tempRunes = tempRunes[:0]

		// collect and process current segment
		segment := trimSpaceRunes(currentRunes[startPos : splitPos+1])

		// check if segment meets minimum length and ends with punctuation
		if len(segment) >= minLen && punctuationMap[segment[len(segment)-1]] {
			sentences = append(sentences, string(segment))
		} else {
			// if not meeting conditions, add to remaining
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

func main() {
	text := `Hey, I know you're just brushing me off again! Every time I ask you, you don't even like me? Hmph, I'm going to be angry! Not friends with you! Unless... you promise me, take me to the night market for tofu pudding~ or hold my hand and stroll around the big street, all the way you have to make me laugh, make me so happy I could fly to the sky! Otherwise I really won't talk to you~`
	sentences, remaining := extractSmartSentences(text, 3, 200)
	for i, sentence := range sentences {
		fmt.Printf("\nSentence %d:\n%s\n", i+1, sentence)
	}
	if remaining != "" {
		fmt.Printf("\nRemaining:\n%s\n", remaining)
	}
}
