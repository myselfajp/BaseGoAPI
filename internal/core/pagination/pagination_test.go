package pagination

import "testing"

func TestCalculateOffset(t *testing.T) {
	cases := []struct {
		name  string
		page  int
		limit int
		want  int
	}{
		{"first page has zero offset", 1, 20, 0},
		{"third page with limit 20", 3, 20, 40},
		{"second page with limit 10", 2, 10, 10},
		{"page below one clamps to zero offset", 0, 20, 0},
		{"negative page clamps to zero offset", -5, 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateOffset(tc.page, tc.limit)
			if got != tc.want {
				t.Errorf("CalculateOffset(%d, %d) = %d, want %d", tc.page, tc.limit, got, tc.want)
			}
		})
	}
}

func TestCalculateTotalPages(t *testing.T) {
	cases := []struct {
		name  string
		total int
		limit int
		want  int
	}{
		{"zero total yields zero pages", 0, 20, 0},
		{"negative total yields zero pages", -1, 20, 0},
		{"exact division", 100, 20, 5},
		{"remainder rounds up", 101, 20, 6},
		{"single item rounds up to one page", 1, 20, 1},
		{"limit is full total", 20, 20, 1},
		{"non-positive limit yields zero pages", 100, 0, 0},
		{"negative limit yields zero pages", 100, -5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateTotalPages(tc.total, tc.limit)
			if got != tc.want {
				t.Errorf("CalculateTotalPages(%d, %d) = %d, want %d", tc.total, tc.limit, got, tc.want)
			}
		})
	}
}

func TestNewMeta(t *testing.T) {
	cases := []struct {
		name           string
		total          int
		page           int
		limit          int
		wantTotalPages int
	}{
		{"exact division", 100, 1, 20, 5},
		{"remainder rounds up", 101, 2, 20, 6},
		{"empty result set", 0, 1, 20, 0},
		{"single item", 1, 1, 20, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewMeta(tc.total, tc.page, tc.limit)
			if got.Total != tc.total {
				t.Errorf("NewMeta(%d, %d, %d).Total = %d, want %d", tc.total, tc.page, tc.limit, got.Total, tc.total)
			}
			if got.Page != tc.page {
				t.Errorf("NewMeta(%d, %d, %d).Page = %d, want %d", tc.total, tc.page, tc.limit, got.Page, tc.page)
			}
			if got.Limit != tc.limit {
				t.Errorf("NewMeta(%d, %d, %d).Limit = %d, want %d", tc.total, tc.page, tc.limit, got.Limit, tc.limit)
			}
			if got.TotalPages != tc.wantTotalPages {
				t.Errorf("NewMeta(%d, %d, %d).TotalPages = %d, want %d", tc.total, tc.page, tc.limit, got.TotalPages, tc.wantTotalPages)
			}
		})
	}
}
