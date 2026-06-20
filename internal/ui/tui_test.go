package ui

import "testing"

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		mode  searchMode
		want  string
	}{
		{
			name:  "plain query defaults to title filter",
			query: "zettel",
			mode:  searchModeTitle,
			want:  "title:(zettel)",
		},
		{
			name:  "multi-word title query stays grouped",
			query: "zettel productive",
			mode:  searchModeTitle,
			want:  "title:(zettel productive)",
		},
		{
			name:  "all mode leaves plain query alone",
			query: "zettel productive",
			mode:  searchModeAll,
			want:  "zettel productive",
		},
		{
			name:  "legacy title alias is harmless in title mode",
			query: "t: zettel productive",
			mode:  searchModeTitle,
			want:  "title:(zettel productive)",
		},
		{
			name:  "legacy title alias is removed in all mode",
			query: "t: zettel productive",
			mode:  searchModeAll,
			want:  "zettel productive",
		},
		{
			name:  "long title filter is harmless in title mode",
			query: "title:zettel",
			mode:  searchModeTitle,
			want:  "title:(zettel)",
		},
		{
			name:  "long title filter is removed in all mode",
			query: "title:zettel",
			mode:  searchModeAll,
			want:  "zettel",
		},
		{
			name:  "explicit body filter is preserved",
			query: "body: zettel",
			mode:  searchModeTitle,
			want:  "body: zettel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchQuery(tt.query, tt.mode)
			if got != tt.want {
				t.Fatalf("buildSearchQuery(%q, %v) = %q, want %q", tt.query, tt.mode, got, tt.want)
			}
		})
	}
}

func TestNormalizeInitialSearchText(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "legacy short title prefix is stripped",
			query: "t: zettel productive",
			want:  "zettel productive",
		},
		{
			name:  "legacy long title prefix is stripped",
			query: "title:zettel",
			want:  "zettel",
		},
		{
			name:  "plain query is unchanged",
			query: "zettel",
			want:  "zettel",
		},
		{
			name:  "non-title filter is unchanged",
			query: "body: zettel",
			want:  "body: zettel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeInitialSearchText(tt.query)
			if got != tt.want {
				t.Fatalf("normalizeInitialSearchText(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}
