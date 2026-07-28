package linkbuilder

import "testing"

func TestBuild(t *testing.T) {
	l := Build("https://img.li", "aB3xK9mQ2wZp", "png", "shot.png")
	if l.URL != "https://img.li/i/aB3xK9mQ2wZp.png" {
		t.Errorf("URL=%q", l.URL)
	}
	if l.ThumbnailURL != "https://img.li/t/aB3xK9mQ2wZp.jpg" {
		t.Errorf("Thumb=%q", l.ThumbnailURL)
	}
	if l.Markdown != "![shot.png](https://img.li/i/aB3xK9mQ2wZp.png)" {
		t.Errorf("MD=%q", l.Markdown)
	}
	if l.HTML != `<img src="https://img.li/i/aB3xK9mQ2wZp.png" alt="shot.png">` {
		t.Errorf("HTML=%q", l.HTML)
	}
	if l.BBCode != "[img]https://img.li/i/aB3xK9mQ2wZp.png[/img]" {
		t.Errorf("BB=%q", l.BBCode)
	}
}

func TestBuildEscapesNameInHTML(t *testing.T) {
	l := Build("https://img.li", "k", "png", `a"><script>`)
	if l.HTML != `<img src="https://img.li/i/k.png" alt="a&#34;&gt;&lt;script&gt;">` {
		t.Errorf("HTML 未转义: %q", l.HTML)
	}
}
