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
	// validdivide符set（可自定义extend）
	splitTokens := []rune{'。', '！', '？', '；', '\n', '.', '!', '?', ';'}

	current := []rune(text)
	for len(current) >= minLen {
		// calculate current窗口size
		windowSize := maxLen
		if windowSize > len(current) {
			windowSize = len(current)
		}

		// atvalid窗口in寻找dividepoint
		splitPos := -1
		for i := windowSize - 1; i >= minLen-1; i-- {
			if containsRune(splitTokens, current[i]) {
				splitPos = i
				break
			}
		}

		if splitPos == -1 {
			break // not找tovaliddividepoint
		}

		// divideandsavevalid句child
		sentences = append(sentences, string(current[:splitPos+1]))
		current = current[splitPos+1:]
	}

	return
}

func main() {
	text := "large家好！今天天气no错。我们a起learning自然languageprocess。这个例childdemotextdividefunction。"
	sentences, remaining := extractSmartSentences(text, 3, 20)
	fmt.Println(sentences)
	fmt.Println(remaining)
}
