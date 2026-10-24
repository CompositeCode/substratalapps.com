package core

import (
	"strings"

	"github.com/CompositeCode/substratalapps.com/internal/db"
	"github.com/CompositeCode/substratalapps.com/internal/httpx"
)

// Optional list counts (Conventions → Pagination → include=total).

// listTotal counts a list's matching rows for ?include=total, up to one
// past the cap. from is the list's FROM clause with its joins, and where
// its filters without the cursor position, so every page reports the same
// total. Nil when the caller didn't ask.
func (c *Call) listTotal(from string, where []string, args db.Args) (*int64, error) {
	if want, err := httpx.WantTotal(c.r); err != nil || !want {
		return nil, err
	}
	cond := "true"
	if len(where) > 0 {
		cond = strings.Join(where, " and ")
	}
	var n int64
	if err := db.One(c.ctx, c.q, &n, httpx.CountSQL("select 1 from "+from+" where "+cond), args); err != nil {
		return nil, err
	}
	return &n, nil
}

// visibleTotal counts the rows keep accepts, stopping one past the cap,
// for lists whose visibility is decided in Go rather than SQL.
func visibleTotal[T any](rows []T, keep func(T) (bool, error)) (*int64, error) {
	var n int64
	for _, x := range rows {
		ok, err := keep(x)
		if err != nil {
			return nil, err
		}
		if ok {
			if n++; n > httpx.TotalCap {
				break
			}
		}
	}
	return &n, nil
}

// emptyList answers a list the caller can see nothing in, with a total of
// 0 when one was asked for.
func emptyList[T any](c *Call) error {
	want, err := httpx.WantTotal(c.r)
	if err != nil {
		return err
	}
	l := httpx.List[T]{Data: []T{}}
	if want {
		var zero int64
		l = l.WithTotal(&zero)
	}
	return c.OK(l)
}
