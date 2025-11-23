package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

func GenerateRandomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func RemoveDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}
	for _, item := range slice {
		if _, value := keys[item]; !value {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}

func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

func SanitizeFilename(filename string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(filename)
}

func GetFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}
	return ""
}

func IsValidEmail(email string) bool {
	if !strings.Contains(email, "@") {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

func NormalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", " ")
	
	lines := strings.Split(text, "\n")
	var normalizedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			normalizedLines = append(normalizedLines, line)
		}
	}
	
	return strings.Join(normalizedLines, "\n")
}

func CalculateReadingTime(text string) int {
	words := strings.Fields(text)
	wordCount := len(words)
	if wordCount == 0 {
		return 0
	}
	
	readingTime := wordCount / 200
	if wordCount%200 > 0 {
		readingTime++
	}
	
	return readingTime
}

func ExtractKeywords(text string, maxKeywords int) []string {
	words := strings.Fields(strings.ToLower(text))
	wordFreq := make(map[string]int)
	
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}<>")
		if len(word) > 3 && !isCommonWord(word) {
			wordFreq[word]++
		}
	}
	
	type keyword struct {
		word  string
		count int
	}
	
	var keywords []keyword
	for word, count := range wordFreq {
		keywords = append(keywords, keyword{word, count})
	}
	
	for i := 0; i < len(keywords)-1; i++ {
		for j := i + 1; j < len(keywords); j++ {
			if keywords[i].count < keywords[j].count {
				keywords[i], keywords[j] = keywords[j], keywords[i]
			}
		}
	}
	
	var result []string
	for i := 0; i < len(keywords) && i < maxKeywords; i++ {
		result = append(result, keywords[i].word)
	}
	
	return result
}

func isCommonWord(word string) bool {
	commonWords := []string{
		"the", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by", "from", "up", "about", "into", "through", "during", "before", "after", "above", "below", "between", "among", "through", "during", "before", "after", "above", "below", "between", "among",
		"is", "are", "was", "were", "be", "been", "being", "have", "has", "had", "do", "does", "did", "will", "would", "could", "should", "may", "might", "must", "can", "shall",
		"a", "an", "this", "that", "these", "those", "i", "you", "he", "she", "it", "we", "they", "me", "him", "her", "us", "them",
		"my", "your", "his", "her", "its", "our", "their", "mine", "yours", "hers", "ours", "theirs",
	}
	
	return Contains(commonWords, word)
}