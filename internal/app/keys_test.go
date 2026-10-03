package app

import "testing"

// The menu accepts digits 1 to 9, and nothing else. A row number beyond the end
// of the list has to be rejected rather than silently starting nothing.
func TestDigitIndexOnlyAcceptsRowsThatExist(t *testing.T) {
	for _, c := range []struct {
		key     string
		count   int
		want    int
		wantOK  bool
		comment string
	}{
		{key: "1", count: 3, want: 0, wantOK: true},
		{key: "3", count: 3, want: 2, wantOK: true},
		{key: "4", count: 3, wantOK: false, comment: "one past the last row"},
		{key: "9", count: 20, want: 8, wantOK: true},
		{key: "0", count: 3, wantOK: false, comment: "rows are numbered from one"},
		{key: "a", count: 3, wantOK: false, comment: "not a digit"},
		{key: "up", count: 3, wantOK: false, comment: "a named key, not a digit"},
		{key: "ctrl+c", count: 3, wantOK: false, comment: "a modifier chord"},
		{key: "", count: 3, wantOK: false, comment: "empty"},
	} {
		t.Run(c.comment, func(t *testing.T) {
			got, ok := digitIndex(c.key, c.count)

			if ok != c.wantOK {
				t.Fatalf("digitIndex(%q, %d) ok = %v, want %v", c.key, c.count, ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Errorf("digitIndex(%q, %d) = %d, want %d", c.key, c.count, got, c.want)
			}
		})
	}
}

func TestDigitIndexRejectsAnythingWithNoRowsToChoose(t *testing.T) {
	if _, ok := digitIndex("1", 0); ok {
		t.Error("digitIndex(\"1\", 0) = ok, want false; there is no first row")
	}
}

func TestTheQuitKeysAreRecognised(t *testing.T) {
	for _, c := range []struct {
		key  string
		want bool
	}{
		{"q", true},
		{"ctrl+c", true},
		{"esc", false},
		{"Q", false},
		{"", false},
	} {
		if got := isQuit(c.key); got != c.want {
			t.Errorf("isQuit(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestTheArrowKeysHaveVimEquivalents(t *testing.T) {
	for _, c := range []struct {
		key  string
		want bool
	}{
		{"up", true},
		{"k", true},
		{"down", true},
		{"j", true},
		{"left", false},
		{"x", false},
	} {
		t.Run(c.key, func(t *testing.T) {
			if up, down := isUp(c.key), isDown(c.key); up != c.want && down != c.want {
				t.Errorf("isUp(%q) = %v, isDown(%q) = %v; want one of them true", c.key, up, c.key, down)
			}
			if isUp(c.key) && isDown(c.key) {
				t.Errorf("%q is both up and down", c.key)
			}
		})
	}
}
