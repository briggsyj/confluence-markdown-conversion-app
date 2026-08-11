package page

import "testing"

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Hello World", "Hello World"},
		{"reserved chars become spaces", `A/B\C:D`, "A B C D"},
		{"collapses runs of spaces", "A   B", "A B"},
		{"punctuation list", `Q&A: "Report" (v2) [final]`, "Q A Report v2 final "},
		{"does not trim edges", " Leading", " Leading"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeFilename(tc.in); got != tc.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestConfluenceIDFromBasename(t *testing.T) {
	if id, ok := ConfluenceIDFromBasename("index"); ok || id != "" {
		t.Errorf("index basename should yield no ID, got (%q, %v)", id, ok)
	}
	if id, ok := ConfluenceIDFromBasename("My Page_123456789"); !ok || id != "123456789" {
		t.Errorf("expected trailing ID 123456789, got (%q, %v)", id, ok)
	}
	if id, ok := ConfluenceIDFromBasename("no-digits-here"); ok {
		t.Errorf("expected no match for basename with no trailing digits, got (%q, %v)", id, ok)
	}
}

func TestLocalDirFromSegments(t *testing.T) {
	got := LocalDirFromSegments([]string{"Team Docs", "Onboarding: FAQs"})
	want := "Team Docs/Onboarding FAQs"
	if got != want {
		t.Errorf("LocalDirFromSegments = %q, want %q", got, want)
	}
	if got := LocalDirFromSegments(nil); got != "" {
		t.Errorf("LocalDirFromSegments(nil) = %q, want empty string", got)
	}
}

func TestFileNameNew(t *testing.T) {
	if got := FileNameNew(true, "ignored"); got != "index.md" {
		t.Errorf("index page FileNameNew = %q, want index.md", got)
	}
	if got := FileNameNew(false, "My Heading!"); got != "My Heading .md" {
		t.Errorf("FileNameNew = %q, want %q", got, "My Heading .md")
	}
}

func TestSpacePath(t *testing.T) {
	got := SpacePath("Team Space", "My Heading.md")
	want := "../Team Space/My Heading.md"
	if got != want {
		t.Errorf("SpacePath = %q, want %q", got, want)
	}
}
