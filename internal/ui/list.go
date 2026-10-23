package ui

import (
	"sort"
	"strings"

	"addresspad/internal/model"
)

func sortedServers(servers []model.Server) []model.Server {
	result := append([]model.Server{}, servers...)
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Favorite != b.Favorite {
			return a.Favorite
		}
		if !a.LastUsed.Equal(b.LastUsed) {
			return a.LastUsed.After(b.LastUsed)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result
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
