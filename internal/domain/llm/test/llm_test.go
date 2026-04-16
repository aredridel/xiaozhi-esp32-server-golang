package main

import (
	"fmt"
)

func containsRune(slice []rune, target rune) bool {
	for _, r := range slice {
		if r == target {
			return true
		}
	}
	return false
}

func extractSmartSentences(text string, minLen, maxLen int) (sentences []string, remaining string) {
	// valid split token set (can custom extend)
	splitTokens := []rune{'。', '！', '？', '；', '\n', '.', '!', '?', ';'}

	current := []rune(text)
	for len(current) >= minLen {
		// calculate current window size
		windowSize := maxLen
		if windowSize > len(current) {
			windowSize = len(current)
		}

		// at valid window in find split point
		splitPos := -1
		for i := windowSize - 1; i >= minLen-1; i-- {
			if containsRune(splitTokens, current[i]) {
				splitPos = i
				break
			}
		}

		if splitPos == -1 {
			break // not find valid split point
		}

		// split and save valid sentence
		sentences = append(sentences, string(current[:splitPos+1]))
		current = current[splitPos+1:]
	}

	return
}

func main() {
	text := "Hello everyone! The weather is nice today. Let's learn natural language processing together. This example demonstrates text segmentation function."
	sentences, remaining := extractSmartSentences(text, 3, 20)
	fmt.Println(sentences)
	fmt.Println(remaining)
}
