package intake

import "testing"

// #88: the extraction that started this proposed thirteen categories for one
// 640-word career, with three near-duplicate clusters. Either the count or the
// word-sharing alone is enough to want the whole set reviewed together.
func TestIsCrowded(t *testing.T) {
	cases := []struct {
		name     string
		proposed []string
		want     bool
	}{
		{
			name:     "the #88 set",
			proposed: []string{"Quality Improvement", "Quality and Safety", "Process Improvement", "Education and Training", "Implementation and Training", "Patient Education", "Clinical Operations", "Clinical Processes", "Charting Systems", "Certifications", "Care Coordination", "Staffing", "Committees"},
			want:     true,
		},
		{
			name:     "few, but two share a word",
			proposed: []string{"Quality Improvement", "Process Improvement", "Clinical"},
			want:     true,
		},
		{
			name:     "nine distinct names trips on count alone",
			proposed: []string{"Clinical", "Charting", "Certifications", "Stations", "Cuisines", "Equipment", "Suppliers", "Events", "Menus"},
			want:     true,
		},
		{
			name:     "a normal small set",
			proposed: []string{"Clinical", "Charting Systems", "Certifications"},
			want:     false,
		},
		{
			name:     "only a short word (of) in common does not count",
			proposed: []string{"Front of House", "Heart of Kitchen", "Bar"},
			want:     false,
		},
		{
			name:     "empty",
			proposed: nil,
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCrowded(tc.proposed); got != tc.want {
				t.Errorf("isCrowded(%v) = %v, want %v", tc.proposed, got, tc.want)
			}
		})
	}
}

func TestHasSharedToken(t *testing.T) {
	cases := []struct {
		name  string
		names []string
		want  bool
	}{
		{"improvement twice", []string{"Quality Improvement", "Process Improvement"}, true},
		{"quality twice", []string{"Quality Improvement", "Quality and Safety"}, true},
		{"clinical twice", []string{"Clinical Operations", "Clinical Processes"}, true},
		{"case-insensitive", []string{"PATIENT education", "patient Education"}, true},
		{"no overlap", []string{"Clinical", "Charting Systems", "Certifications"}, false},
		{"only the short word 'of' in common", []string{"Front of House", "Heart of Kitchen"}, false},
		{"a real word in common still counts", []string{"Front of House", "Back of House"}, true},
		{"single name cannot self-collide", []string{"Quality Improvement Improvement"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasSharedToken(tc.names); got != tc.want {
				t.Errorf("hasSharedToken(%v) = %v, want %v", tc.names, got, tc.want)
			}
		})
	}
}
