package icons

import "testing"

func TestFor(t *testing.T) {
	cases := []struct {
		name  string
		dir   bool
		open  bool
		glyph string
	}{
		{"main.go", false, false, ""},
		{"go.mod", false, false, ""},
		{"NOTES.MD", false, false, ""},
		{"README.md", false, false, ""},
		{"LICENSE", false, false, ""},
		{"mygo.json", false, false, ""},
		{"app.ts", false, false, ""},
		{"x.js", false, false, ""},
		{"run.py", false, false, ""},
		{"build.sh", false, false, ""},
		{"gradlew.bat", false, false, ""},
		{"build.gradle.kts", false, false, ""},
		{"a.zip", false, false, ""},
		{"photo.JPG", false, false, ""},
		{"todo.txt", false, false, ""},
		{".env.local", false, false, Hidden.Glyph},
		{".gitignore", false, false, ""},
		{"unknown.qqq", false, false, File.Glyph},
		{"noext", false, false, File.Glyph},
		{"src", true, false, DirClosed.Glyph},
		{"src", true, true, DirOpen.Glyph},
		{"Downloads", true, false, ""},
		{"Downloads", true, true, DirOpen.Glyph},
	}
	for _, c := range cases {
		if got := For(c.name, c.dir, c.open); got.Glyph != c.glyph {
			t.Errorf("For(%q, %v, %v) = %q, want %q", c.name, c.dir, c.open, got.Glyph, c.glyph)
		}
	}
}

// TestKinds checks one name of each kind phase 8 added.
func TestKinds(t *testing.T) {
	cases := map[string]string{
		"lib.rs": "", "a.rb": "", "i.php": "", "x.lua": "", "P.cs": "\U000f031b",
		"m.swift": "", "App.vue": "\U000f0844", "c.yaml": "", "d.xml": "\U000f05c0",
		"r.pdf": "", "w.docx": "", "s.xlsx": "", "t.csv": "", "k.key": "", "p.pptx": "",
		"a.svg": "\U000f0721", "v.mp4": "", "s.flac": "", "b.tar.gz": "", "d.dmg": "",
		"f.ttf": "", "c.pem": "", "k.p12": "", "id_rsa": "", "a.log": "",
		"x.sqlite": "", "q.sql": "", "Dockerfile": "", "Cargo.toml": "", ".zshrc": "",
	}
	for name, want := range cases {
		if got := For(name, false, false).Glyph; got != want {
			t.Errorf("For(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSpecial(t *testing.T) {
	cases := []struct {
		name string
		at   Attr
		want Icon
	}{
		{"Safari.app", Attr{Dir: true}, App},
		{"Proj.xcodeproj", Attr{Dir: true}, byDirExt["xcodeproj"]},
		{"tool", Attr{Exec: true}, Exec},
		{"notes.md", Attr{Exec: true}, byExt["md"]}, // a known kind wins over x
		{"ln", Attr{Link: true}, LinkFile},
		{"lnd", Attr{Link: true, Dir: true}, LinkDir},
		{"repo", Attr{Dir: true, Repo: true}, Repo},
		{".git", Attr{Dir: true}, byDir[".git"]},
		{"node_modules", Attr{Dir: true}, byDir["node_modules"]},
		{".config", Attr{Dir: true}, byDir[".config"]},
		{"Desktop", Attr{Dir: true}, byDir["desktop"]},
		{"iCloud Drive", Attr{Dir: true}, byDir["icloud drive"]},
	}
	for _, c := range cases {
		got := Of(c.name, c.at)
		if got.Glyph != c.want.Glyph || got.Kind != c.want.Kind {
			t.Errorf("Of(%q, %+v) = %q, want %q", c.name, c.at, got.Glyph, c.want.Glyph)
		}
	}
	if Repo.Glyph == byDir[".git"].Glyph {
		t.Error("a repository's root looks like its .git folder")
	}
}

func TestHiddenDim(t *testing.T) {
	if !Of(".zshrc", Attr{}).Dim || !Of(".git", Attr{Dir: true}).Dim || Of("main.go", Attr{}).Dim {
		t.Fatal("only hidden items are dim")
	}
}

// TestKindOf checks the package returns categories, not colors.
func TestKindOf(t *testing.T) {
	cases := []struct {
		name string
		at   Attr
		want Kind
	}{
		{"src", Attr{Dir: true}, KindFolder},
		{"src", Attr{Dir: true, Open: true}, KindFolder},
		{"Safari.app", Attr{Dir: true}, KindFolder},
		{"main.go", Attr{}, KindCode},
		{"run.sh", Attr{}, KindCode},
		{"notes.md", Attr{}, KindDoc},
		{"a.pdf", Attr{}, KindDoc},
		{"p.png", Attr{}, KindImage},
		{"v.mp4", Attr{}, KindMedia},
		{"s.flac", Attr{}, KindMedia},
		{"a.zip", Attr{}, KindArchive},
		{"tool", Attr{Exec: true}, KindExec},
		{"c.yaml", Attr{}, KindConfig},
		{".zshrc", Attr{}, KindConfig},
		{".hidden", Attr{}, KindConfig},
		{"plain", Attr{}, KindDoc},
	}
	for _, c := range cases {
		if got := Of(c.name, c.at).Kind; got != c.want {
			t.Errorf("Of(%q, %+v).Kind = %d, want %d", c.name, c.at, got, c.want)
		}
	}
}

func TestFallback(t *testing.T) {
	if Fallback(true, false) != "▸" || Fallback(true, true) != "▾" || Fallback(false, false) != " " {
		t.Fatal("fallback symbols changed")
	}
}
