package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestCursors(t *testing.T) {
	c := Cursors{Key: []byte("k")}
	cur := c.Encode("2026-10-05T00:00:00Z", "usr_1", "f1")
	pos, err := c.Decode(cur, "f1")
	if err != nil || pos.ID != "usr_1" {
		t.Fatalf("decode = %v, %v", pos, err)
	}
	if _, err := c.Decode(cur, "f2"); err == nil {
		t.Fatal("a cursor is only valid with its own filters")
	}
	if _, err := c.Decode(cur[:len(cur)-2]+"xx", "f1"); err == nil {
		t.Fatal("a tampered cursor is invalid")
	}
	if _, err := (Cursors{Key: []byte("other")}).Decode(cur, "f1"); err == nil {
		t.Fatal("a cursor from another key is invalid")
	}
}

func TestLimitAndFilters(t *testing.T) {
	for in, want := range map[int]int{0: 1, 25: 25, 500: 100} {
		v := in
		if got := Limit(&v); got != want {
			t.Errorf("Limit(%d) = %d", in, got)
		}
	}
	if Limit(nil) != 25 {
		t.Error("default limit is 25")
	}
	a := Filters(httptest.NewRequest("GET", "/users?status=active&limit=5&cursor=x", nil))
	b := Filters(httptest.NewRequest("GET", "/users?status=active", nil))
	c := Filters(httptest.NewRequest("GET", "/users?status=invited", nil))
	if a != b || a == c {
		t.Fatal("filters ignore limit/cursor and nothing else")
	}
}

func TestIfMatch(t *testing.T) {
	r := httptest.NewRequest("PATCH", "/", nil)
	if CheckIfMatch(r, 3) != nil {
		t.Fatal("absent If-Match is last-write-wins")
	}
	r.Header.Set("If-Match", `W/"3"`)
	if CheckIfMatch(r, 3) != nil || CheckIfMatch(r, 4) == nil {
		t.Fatal("If-Match comparison")
	}
}

func TestTotals(t *testing.T) {
	for q, want := range map[string]bool{"": false, "?include=total": true} {
		got, err := WantTotal(httptest.NewRequest("GET", "/users"+q, nil))
		if err != nil || got != want {
			t.Errorf("WantTotal(%q) = %v, %v", q, got, err)
		}
	}
	if _, err := WantTotal(httptest.NewRequest("GET", "/users?include=count", nil)); err == nil {
		t.Error("an unknown include value is a validation error")
	}
	if Filters(httptest.NewRequest("GET", "/users?status=active&include=total", nil)) !=
		Filters(httptest.NewRequest("GET", "/users?status=active", nil)) {
		t.Error("include isn't part of a cursor's filters")
	}
	for n, want := range map[int64][2]any{0: {int64(0), false}, 10000: {int64(10000), false}, 10001: {int64(10000), true}} {
		v := n
		l := List[int]{}.WithTotal(&v)
		if *l.Page.Total != want[0] || *l.Page.TotalCapped != want[1] {
			t.Errorf("WithTotal(%d) = %d, %v", n, *l.Page.Total, *l.Page.TotalCapped)
		}
	}
	if l := (List[int]{}).WithTotal(nil); l.Page.Total != nil || l.Page.TotalCapped != nil {
		t.Error("no count, no total")
	}
}
