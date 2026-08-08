package adminsvc

import "testing"

func TestNormalizeThemeAccent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  #3B82F6  ", "#3b82f6"},
		{"#abc", "#aabbcc"},
		{"#ABC", "#aabbcc"},
	}
	for _, c := range cases {
		if got := NormalizeThemeAccent(c.in); got != c.want {
			t.Errorf("NormalizeThemeAccent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateThemeAccent(t *testing.T) {
	if err := ValidateThemeAccent(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeAccent("#3b82f6"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeAccent("#fff"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"red", "#gg0000", "#12", "3b82f6", "#12345"} {
		if err := ValidateThemeAccent(bad); err == nil {
			t.Errorf("ValidateThemeAccent(%q) should fail", bad)
		}
	}
}

func TestThemeBgColor(t *testing.T) {
	if got := NormalizeThemeBgColor("#ABC"); got != "#aabbcc" {
		t.Errorf("NormalizeThemeBgColor = %q", got)
	}
	if err := ValidateThemeBgColor(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeBgColor("#112233"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeBgColor("nope"); err == nil {
		t.Error("invalid bg color should fail")
	}
}

func TestValidateThemeBgDim(t *testing.T) {
	if err := ValidateThemeBgDim(0); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeBgDim(1); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeBgDim(0.72); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeBgDim(-0.1); err == nil {
		t.Error("negative dim should fail")
	}
	if err := ValidateThemeBgDim(1.01); err == nil {
		t.Error(">1 dim should fail")
	}
}

func TestValidateThemeGlass(t *testing.T) {
	if err := ValidateThemeGlass(0.78); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeGlass(0); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeGlass(1.2); err == nil {
		t.Error(">1 glass should fail")
	}
}

func TestContrastOnAccent(t *testing.T) {
	if got := ContrastOnAccent("#000000"); got != "#ffffff" {
		t.Errorf("dark accent text = %q", got)
	}
	if got := ContrastOnAccent("#ffffff"); got != "#17171a" {
		t.Errorf("light accent text = %q", got)
	}
	if got := ContrastOnAccent("#3b82f6"); got != "#ffffff" {
		t.Errorf("blue accent text = %q", got)
	}
}
