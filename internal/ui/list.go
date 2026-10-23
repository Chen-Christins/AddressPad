package ui

import (
	"strings"

	"addresspad/internal/model"
)

func sortedServers(servers []model.Server) []model.Server {
	return model.Sorted(servers)
}

func subsequence(needle, haystack string) bool {
	n := []rune(strings.ToLower(needle))
	pos := 0
	for _, r := range strings.ToLower(haystack) {
		if pos < len(n) && n[pos] == r {
			pos++
		}
	}
	return pos == len(n)
}

// Each keyword must match at least one field, without crossing field boundaries.
func filterServers(servers []model.Server, query string) []model.Server {
	words := strings.Fields(query)
	result := make([]model.Server, 0, len(servers))
	for _, s := range servers {
		fields := []string{s.Name, s.User, s.Host, s.Group}
		fields = append(fields, s.Tags...)
		matches := true
		for _, word := range words {
			found := false
			for _, field := range fields {
				if subsequence(word, field) {
					found = true
					break
				}
			}
			if !found {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, s)
		}
	}
	return result
}
