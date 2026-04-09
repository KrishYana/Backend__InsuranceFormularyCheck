package handler

import (
	"testing"
)

func TestTopNByCount(t *testing.T) {
	tests := []struct {
		name   string
		counts map[int]int
		n      int
		wantN  int      // expected number of results
		wantID []int    // expected IDs in order (top N by count descending)
	}{
		{
			name:   "basic top 3",
			counts: map[int]int{1: 10, 2: 5, 3: 20, 4: 15, 5: 1},
			n:      3,
			wantN:  3,
			wantID: []int{3, 4, 1},
		},
		{
			name:   "n larger than map",
			counts: map[int]int{1: 5, 2: 10},
			n:      5,
			wantN:  2,
		},
		{
			name:   "empty map",
			counts: map[int]int{},
			n:      5,
			wantN:  0,
		},
		{
			name:   "n is zero",
			counts: map[int]int{1: 10, 2: 5},
			n:      0,
			wantN:  0,
		},
		{
			name:   "single element",
			counts: map[int]int{42: 100},
			n:      5,
			wantN:  1,
			wantID: []int{42},
		},
		{
			name:   "top 1",
			counts: map[int]int{1: 3, 2: 7, 3: 1},
			n:      1,
			wantN:  1,
			wantID: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := topNByCount(tt.counts, tt.n)

			if len(result) != tt.wantN {
				t.Errorf("topNByCount returned %d items, want %d", len(result), tt.wantN)
				return
			}

			// Verify ordering: each item should have count >= next item
			for i := 1; i < len(result); i++ {
				if result[i].count > result[i-1].count {
					t.Errorf("results not sorted: item %d (count=%d) > item %d (count=%d)",
						i, result[i].count, i-1, result[i-1].count)
				}
			}

			// If we have specific expected IDs, check them
			if tt.wantID != nil {
				for i, wantID := range tt.wantID {
					if i >= len(result) {
						break
					}
					if result[i].id != wantID {
						t.Errorf("result[%d].id = %d, want %d", i, result[i].id, wantID)
					}
				}
			}
		})
	}
}

func TestTopNByCount_Deterministic(t *testing.T) {
	// With equal counts, the order may vary, but the function should not panic
	// and should always return the correct number of results
	counts := map[int]int{1: 5, 2: 5, 3: 5, 4: 5, 5: 5}

	result := topNByCount(counts, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d", len(result))
	}

	// All should have count 5
	for i, item := range result {
		if item.count != 5 {
			t.Errorf("result[%d].count = %d, want 5", i, item.count)
		}
	}
}

func TestWeekKey(t *testing.T) {
	tests := []struct {
		name string
		year int
		week int
		want string
	}{
		{
			name: "standard week",
			year: 2025,
			week: 14,
			want: "2025-W14",
		},
		{
			name: "single digit week",
			year: 2025,
			week: 1,
			want: "2025-W01",
		},
		{
			name: "week 52",
			year: 2024,
			week: 52,
			want: "2024-W52",
		},
		{
			name: "week 53 (leap year)",
			year: 2020,
			week: 53,
			want: "2020-W53",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := weekKey(tt.year, tt.week)
			if got != tt.want {
				t.Errorf("weekKey(%d, %d) = %q, want %q", tt.year, tt.week, got, tt.want)
			}
		})
	}
}
