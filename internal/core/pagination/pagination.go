// Package pagination provides helpers for consistent pagination across the
// application. It is the Go equivalent of app/core/pagination.py.
package pagination

// CalculateOffset converts a 1-indexed page and limit into a database offset.
func CalculateOffset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}

// CalculateTotalPages returns the number of pages needed for total items.
func CalculateTotalPages(total, limit int) int {
	if total <= 0 || limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}

// Meta is the pagination metadata attached to list responses.
type Meta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
}

// NewMeta builds pagination metadata with the total page count computed.
func NewMeta(total, page, limit int) Meta {
	return Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: CalculateTotalPages(total, limit),
	}
}
