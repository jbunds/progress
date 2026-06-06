package progress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetTheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want *theme
	}{
		{
			name: "default", // no theme named "default"; should return "sunset"
			want: &theme{
				name:        "sunset",
				transitions: []endpoints{
					{initial: rgb{r:  48, g:  25, b:  52}, final: rgb{r: 199, g:   0, b:  57}},
					{initial: rgb{r: 199, g:   0, b:  57}, final: rgb{r: 255, g:  87, b:  51}},
					{initial: rgb{r: 255, g:  87, b:  51}, final: rgb{r: 255, g: 195, b:   0}},
				},
			},
    },
		{
			name: "ocean",
			want: &theme{
				name:        "ocean",
				transitions: []endpoints{
					{initial: rgb{r:   0, g:  10, b:  45}, final: rgb{r:   0, g: 128, b: 128}},
					{initial: rgb{r:   0, g: 128, b: 128}, final: rgb{r:   0, g: 200, b: 150}},
					{initial: rgb{r:   0, g: 200, b: 150}, final: rgb{r: 100, g: 255, b: 150}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newThemeRegistry().get(tt.name)
			if diff := cmp.Diff(tt.want, got, getCmpOpts()); diff != "" {
				t.Errorf("themeOrDefault(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	theme := &theme{}
	if theme.Equal(nil) {
		t.Errorf("Equal() mismatch: expected false; got true")
	}
}
