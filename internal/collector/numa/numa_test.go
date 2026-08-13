package numa

import (
	"reflect"
	"testing"
)

func TestParseList(t *testing.T) {
	got, err := ParseList("0-2,4,6-7")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2, 4, 6, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
func TestParseListRejectsReverse(t *testing.T) {
	if _, err := ParseList("4-1"); err == nil {
		t.Fatal("expected error")
	}
}

func FuzzParseList(f *testing.F) {
	for _, seed := range []string{"0", "0-3", "0-2,4,6-7", "", "4-1", "x"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		values, err := ParseList(raw)
		if err != nil {
			return
		}
		seen := map[int]bool{}
		for _, value := range values {
			if value < 0 {
				t.Fatalf("negative CPU %d", value)
			}
			if seen[value] {
				t.Fatalf("duplicate CPU %d", value)
			}
			seen[value] = true
		}
	})
}

func TestParseDistance(t *testing.T) {
	got, err := ParseDistance("10 21\n")
	if err != nil || len(got) != 2 || got[1] != 21 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if _, err := ParseDistance("10 bad"); err == nil {
		t.Fatal("expected error")
	}
}
