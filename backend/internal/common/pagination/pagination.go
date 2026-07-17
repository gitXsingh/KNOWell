package pagination

import (
	"net/http"
	"strconv"
)

type Params struct {
	Limit  int
	Offset int
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func FromRequest(r *http.Request) Params {
	limit := DefaultLimit
	offset := 0

	if l := r.URL.Query().Get("per_page"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 1 {
			offset = (n - 1) * limit
		}
	}

	return Params{Limit: limit, Offset: offset}
}
