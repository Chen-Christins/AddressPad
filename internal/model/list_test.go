package model

import (
	"testing"
	"time"
)

func TestSorted(t *testing.T) {
	now := time.Now()
	source := []Server{{Name: "z"}, {Name: "beta"}, {Name: "Alpha"}, {Name: "recent", LastUsed: now}, {Name: "pinned", Favorite: true}}
	got := Sorted(source)
	for i, want := range []string{"pinned", "recent", "Alpha", "beta", "z"} {
		if got[i].Name != want {
			t.Fatalf("sort = %+v", got)
		}
	}
	if source[0].Name != "z" {
		t.Fatal("sort changed source")
	}
	if Sorted(nil) == nil {
		t.Fatal("nil source must produce an empty array")
	}
}
