package model

import (
	"sort"
	"strings"
)

// Sorted returns a detached list ordered by favorite, last use and name.
func Sorted(servers []Server) []Server {
	result := append([]Server{}, servers...)
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
