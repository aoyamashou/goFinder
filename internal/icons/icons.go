// Package icons maps file names to Nerd Font glyphs and colors, after
// Yazi's default icon theme and the nerd-fonts sets (nf-seti, nf-dev,
// nf-md, nf-fa). The glyphs need a Nerd Font installed (the app uses
// JetBrainsMono Nerd Font Mono); without one, Fallback gives plain symbols.
//
// It gives each item a Kind, not a color.
package icons

import (
	"path"
	"strings"
)

// Kind is the category of an item: the UI colors a glyph by its kind
// (the design system's type-* tokens); this package knows no colors.
type Kind uint8

const (
	KindDoc     Kind = iota // documents, text, spreadsheets, fonts
	KindFolder              // folders, bundles, repository roots
	KindCode                // source code and scripts
	KindImage               // pictures and design files
	KindMedia               // video and audio
	KindArchive             // archives, disk images, packages
	KindExec                // programs
	KindConfig              // configuration, locks, keys, logs, dotfiles
)

// Icon is a glyph and its kind. Dim asks for it, and the name beside it,
// to be drawn faded, as hidden items are.
type Icon struct {
	Glyph string
	Kind  Kind
	Dim   bool
}

// Font is the Nerd Font family the lists use; FontAlt is the same font
// with wider glyphs, which the lists fall back to.
const (
	Font    = "JetBrainsMono Nerd Font Mono"
	FontAlt = "JetBrainsMono Nerd Font"
)

var (
	DirClosed = Icon{Glyph: "", Kind: KindFolder} //  nf-fa-folder
	DirOpen   = Icon{Glyph: "", Kind: KindFolder} //  nf-fa-folder_open
	File      = Icon{Glyph: "", Kind: KindDoc}    //  nf-fa-file_o
	Hidden    = Icon{Glyph: "", Kind: KindConfig} //  nf-fa-gear
	Exec      = Icon{Glyph: "", Kind: KindExec}   //  nf-oct-file_binary
	LinkFile  = Icon{Glyph: "", Kind: KindDoc}    //  nf-oct-file_symlink_file
	LinkDir   = Icon{Glyph: "", Kind: KindFolder} //  nf-oct-file_symlink_directory
	Repo      = Icon{Glyph: "", Kind: KindFolder} //  nf-custom-folder_git: a repository's root
	App       = Icon{Glyph: "", Kind: KindFolder} //  nf-fa-rocket: an .app
)

// byName is matched first, by the whole name (lower case).
var byName = map[string]Icon{
	"license":            {Glyph: "", Kind: KindDoc}, //  nf-seti-license
	"license.md":         {Glyph: "", Kind: KindDoc},
	"license.txt":        {Glyph: "", Kind: KindDoc},
	"copying":            {Glyph: "", Kind: KindConfig},
	"readme":             {Glyph: "", Kind: KindDoc}, //  nf-oct-book
	"readme.md":          {Glyph: "", Kind: KindDoc},
	"readme.txt":         {Glyph: "", Kind: KindDoc},
	"changelog.md":       {Glyph: "", Kind: KindDoc}, //  nf-fa-history
	"changelog":          {Glyph: "", Kind: KindDoc},
	"makefile":           {Glyph: "", Kind: KindConfig}, //  nf-dev-gnu
	"gnumakefile":        {Glyph: "", Kind: KindConfig},
	"cmakelists.txt":     {Glyph: "", Kind: KindConfig},
	"justfile":           {Glyph: "", Kind: KindConfig},
	"dockerfile":         {Glyph: "", Kind: KindConfig}, //  nf-linux-docker
	"docker-compose.yml": {Glyph: "", Kind: KindConfig},
	"compose.yaml":       {Glyph: "", Kind: KindConfig},
	".dockerignore":      {Glyph: "", Kind: KindConfig},
	"brewfile":           {Glyph: "", Kind: KindConfig}, //  nf-fa-beer
	"go.mod":             {Glyph: "", Kind: KindConfig}, //  nf-seti-go
	"go.sum":             {Glyph: "", Kind: KindConfig},
	"go.work":            {Glyph: "", Kind: KindConfig},
	"cargo.toml":         {Glyph: "", Kind: KindConfig}, //  nf-dev-rust
	"cargo.lock":         {Glyph: "", Kind: KindConfig},
	"gemfile":            {Glyph: "", Kind: KindConfig}, //  nf-dev-ruby
	"rakefile":           {Glyph: "", Kind: KindConfig},
	".gitignore":         {Glyph: "", Kind: KindConfig}, //  nf-dev-git
	".gitattributes":     {Glyph: "", Kind: KindConfig},
	".gitmodules":        {Glyph: "", Kind: KindConfig},
	".gitconfig":         {Glyph: "", Kind: KindConfig},
	".ds_store":          {Glyph: "", Kind: KindConfig}, //  nf-fa-apple
	".localized":         {Glyph: "", Kind: KindConfig},
	"gradlew":            {Glyph: "", Kind: KindConfig}, //  nf-seti-gradle
	"package.json":       {Glyph: "", Kind: KindConfig}, //  nf-dev-npm
	"package-lock.json":  {Glyph: "", Kind: KindConfig},
	".npmrc":             {Glyph: "", Kind: KindConfig},
	"tsconfig.json":      {Glyph: "", Kind: KindConfig}, //  nf-seti-typescript
	".env":               {Glyph: "", Kind: KindConfig}, //  nf-oct-key
	".zshrc":             {Glyph: "", Kind: KindConfig}, //  nf-oct-terminal
	".zprofile":          {Glyph: "", Kind: KindConfig},
	".bashrc":            {Glyph: "", Kind: KindConfig},
	".bash_profile":      {Glyph: "", Kind: KindConfig},
	".profile":           {Glyph: "", Kind: KindConfig},
	".vimrc":             {Glyph: "", Kind: KindConfig}, //  nf-custom-vim
	"id_rsa":             {Glyph: "", Kind: KindConfig}, //  nf-fa-key
	"id_ed25519":         {Glyph: "", Kind: KindConfig},
	"id_ecdsa":           {Glyph: "", Kind: KindConfig},
	"authorized_keys":    {Glyph: "", Kind: KindConfig},
	"known_hosts":        {Glyph: "", Kind: KindConfig},
}

// byExt is matched by the extension (lower case, without the dot).
var byExt = map[string]Icon{}

// kinds fills byExt: each icon with the extensions that get it.
var kinds = []struct {
	icon Icon
	exts string
}{
	// Languages.
	{Icon{Glyph: "", Kind: KindCode}, "go"},                                            //  nf-seti-go
	{Icon{Glyph: "", Kind: KindCode}, "py pyi pyw ipynb"},                              //  nf-seti-python
	{Icon{Glyph: "", Kind: KindCode}, "ts mts cts"},                                    //  nf-seti-typescript
	{Icon{Glyph: "", Kind: KindCode}, "js mjs cjs"},                                    //  nf-dev-javascript
	{Icon{Glyph: "", Kind: KindCode}, "jsx tsx"},                                       //  nf-dev-react
	{Icon{Glyph: "", Kind: KindCode}, "rs"},                                            //  nf-dev-rust
	{Icon{Glyph: "", Kind: KindCode}, "c m"},                                           //  nf-custom-c
	{Icon{Glyph: "", Kind: KindCode}, "h"},                                             //
	{Icon{Glyph: "", Kind: KindCode}, "cpp cc cxx hpp hh hxx mm"},                      //  nf-custom-cpp
	{Icon{Glyph: "", Kind: KindCode}, "java jar class"},                                //  nf-dev-java
	{Icon{Glyph: "", Kind: KindCode}, "kt kts"},                                        //  nf-seti-kotlin
	{Icon{Glyph: "", Kind: KindCode}, "swift"},                                         //  nf-dev-swift
	{Icon{Glyph: "", Kind: KindCode}, "rb erb gemspec"},                                //  nf-dev-ruby
	{Icon{Glyph: "", Kind: KindCode}, "php"},                                           //  nf-dev-php
	{Icon{Glyph: "", Kind: KindCode}, "lua"},                                           //  nf-seti-lua
	{Icon{Glyph: "\U000f031b", Kind: KindCode}, "cs csx"},                               // 󰌛 nf-md-language_csharp
	{Icon{Glyph: "", Kind: KindCode}, "scala sc"},                                      //  nf-dev-scala
	{Icon{Glyph: "", Kind: KindCode}, "dart"},                                          //  nf-dev-dart
	{Icon{Glyph: "", Kind: KindCode}, "hs lhs"},                                        //  nf-seti-haskell
	{Icon{Glyph: "", Kind: KindCode}, "ex exs eex heex"},                               //  nf-seti-elixir
	{Icon{Glyph: "", Kind: KindCode}, "erl hrl"},                                       //  nf-dev-erlang
	{Icon{Glyph: "", Kind: KindCode}, "clj cljs cljc edn"},                             //  nf-dev-clojure
	{Icon{Glyph: "", Kind: KindCode}, "jl"},                                            //  nf-seti-julia
	{Icon{Glyph: "", Kind: KindCode}, "zig"},                                           //  nf-seti-zig
	{Icon{Glyph: "\U000f0844", Kind: KindCode}, "vue"},                                  // 󰡄 nf-md-vuejs
	{Icon{Glyph: "", Kind: KindCode}, "svelte"},                                        //  nf-seti-svelte
	{Icon{Glyph: "", Kind: KindCode}, "pl pm"},                                         //  nf-dev-perl
	{Icon{Glyph: "", Kind: KindCode}, "sh bash zsh fish ksh command tool bat cmd ps1"}, //  nf-oct-terminal
	{Icon{Glyph: "", Kind: KindCode}, "html htm xhtml"},                                //  nf-dev-html5
	{Icon{Glyph: "", Kind: KindCode}, "css"},                                           //  nf-dev-css3
	{Icon{Glyph: "", Kind: KindCode}, "scss sass"},                                     //  nf-seti-sass
	{Icon{Glyph: "", Kind: KindCode}, "less"},                                          //  nf-dev-less
	{Icon{Glyph: "", Kind: KindDoc}, "md markdown mdx rst"},                            //  nf-dev-markdown
	{Icon{Glyph: "", Kind: KindCode}, "asm s wasm wat glsl metal vim el nim"},          //  nf-fa-code
	{Icon{Glyph: "", Kind: KindCode}, "nix"},                                           //  nf-linux-nixos

	// Configuration.
	{Icon{Glyph: "", Kind: KindConfig}, "json jsonc json5 webmanifest"},                                          //  nf-seti-json
	{Icon{Glyph: "", Kind: KindConfig}, "yaml yml toml ini conf cfg plist env properties editorconfig xcconfig"}, //  nf-seti-config
	{Icon{Glyph: "\U000f05c0", Kind: KindConfig}, "xml xsd xsl"},                                                  // 󰗀 nf-md-xml
	{Icon{Glyph: "", Kind: KindConfig}, "lock"},                                                                  //  nf-fa-lock

	// Documents.
	{Icon{Glyph: "", Kind: KindDoc}, "txt text"},                          //  nf-fa-file_text
	{Icon{Glyph: "", Kind: KindDoc}, "pdf"},                               //  nf-fa-file_pdf_o
	{Icon{Glyph: "", Kind: KindDoc}, "doc docx rtf rtfd odt pages wpd"},   //  nf-fa-file_word_o
	{Icon{Glyph: "", Kind: KindDoc}, "xls xlsx xlsm csv tsv ods numbers"}, //  nf-fa-file_excel_o
	{Icon{Glyph: "", Kind: KindDoc}, "ppt pptx odp key"},                  //  nf-fa-file_powerpoint_o
	{Icon{Glyph: "", Kind: KindDoc}, "epub mobi azw3 djvu"},               //  nf-fa-book
	{Icon{Glyph: "", Kind: KindDoc}, "tex bib sty cls"},                   //  nf-seti-tex

	// Media.
	{Icon{Glyph: "", Kind: KindImage}, "png jpg jpeg gif webp bmp heic heif tif tiff ico icns avif raw cr2 nef dng"}, //  nf-fa-file_image_o
	{Icon{Glyph: "\U000f0721", Kind: KindImage}, "svg"},                                                               // 󰜡 nf-md-svg
	{Icon{Glyph: "", Kind: KindImage}, "psd ai sketch fig xd afdesign afphoto"},                                      //  nf-fa-paint_brush
	{Icon{Glyph: "", Kind: KindMedia}, "mp4 mov mkv avi webm m4v wmv flv mpg mpeg 3gp"},                              //  nf-fa-file_video_o
	{Icon{Glyph: "", Kind: KindMedia}, "mp3 m4a flac wav aac ogg opus aiff aif alac mid midi"},                       //  nf-fa-file_audio_o

	// Archives, disk images, packages.
	{Icon{Glyph: "", Kind: KindArchive}, "zip tar gz tgz bz2 xz 7z rar zst lz lzma z cpio"},     //  nf-oct-file_zip
	{Icon{Glyph: "", Kind: KindArchive}, "dmg iso img sparseimage sparsebundle vhd vmdk qcow2"}, //  nf-fa-hdd_o
	{Icon{Glyph: "", Kind: KindArchive}, "pkg mpkg deb rpm apk ipa xip msi"},                    //  nf-oct-package

	// Fonts, certificates and keys, logs, databases.
	{Icon{Glyph: "", Kind: KindDoc}, "ttf otf ttc woff woff2 eot dfont"},               //  nf-fa-font
	{Icon{Glyph: "", Kind: KindConfig}, "pem crt cer der p7b p7c csr mobileprovision"}, //  nf-fa-certificate
	{Icon{Glyph: "", Kind: KindConfig}, "p12 pfx pub asc gpg sig keychain"},            //  nf-fa-key
	{Icon{Glyph: "", Kind: KindConfig}, "log out"},                                     //  nf-fa-file_text: a log
	{Icon{Glyph: "", Kind: KindConfig}, "db sqlite sqlite3 db3 realm mdb accdb"},       //  nf-fa-database
	{Icon{Glyph: "", Kind: KindCode}, "sql"},                                           //  nf-dev-database

	// Programs.
	{Icon{Glyph: "", Kind: KindExec}, "exe bin dylib so o a wasm"}, //  nf-oct-file_binary
}

// byDir gives some well-known folders their own glyph while closed.
var byDir = map[string]Icon{
	"desktop":             {Glyph: "", Kind: KindFolder}, //  nf-fa-desktop
	"documents":           {Glyph: "", Kind: KindFolder}, //  nf-fa-book
	"downloads":           {Glyph: "", Kind: KindFolder}, //  nf-fa-download
	"music":               {Glyph: "", Kind: KindFolder}, //  nf-fa-headphones
	"movies":              {Glyph: "", Kind: KindFolder}, //  nf-fa-film
	"pictures":            {Glyph: "", Kind: KindFolder}, //  nf-fa-picture_o
	"public":              {Glyph: "", Kind: KindFolder}, //  nf-fa-building
	"library":             {Glyph: "", Kind: KindFolder}, //  nf-fa-database
	"applications":        {Glyph: "", Kind: KindFolder}, //  nf-fa-rocket
	"icloud drive":        {Glyph: "", Kind: KindFolder}, //  nf-fa-cloud
	"mobile documents":    {Glyph: "", Kind: KindFolder},
	"com~apple~clouddocs": {Glyph: "", Kind: KindFolder},
	".trash":              {Glyph: "", Kind: KindFolder},          //  nf-fa-trash
	".git":                {Glyph: "", Kind: KindFolder},          //  nf-dev-git
	".github":             {Glyph: "", Kind: KindFolder},          //  nf-custom-folder_github
	"node_modules":        {Glyph: "", Kind: KindFolder},          //  nf-custom-folder_npm
	".config":             {Glyph: "", Kind: KindFolder},          //  nf-custom-folder_config
	".vscode":             {Glyph: "\U000f0a1e", Kind: KindFolder}, // 󰨞 nf-md-microsoft_visual_studio_code
	".ssh":                {Glyph: "", Kind: KindFolder},          //  nf-fa-key
}

// byDirExt gives folders that macOS shows as one item (bundles) their own
// glyph, by extension.
var byDirExt = map[string]Icon{
	"app":           App,
	"xcodeproj":     {Glyph: "", Kind: KindFolder}, //  nf-dev-swift
	"xcworkspace":   {Glyph: "", Kind: KindFolder},
	"photoslibrary": {Glyph: "", Kind: KindFolder}, //  nf-fa-picture_o
	"framework":     {Glyph: "", Kind: KindFolder}, //  nf-fa-cube
	"bundle":        {Glyph: "", Kind: KindFolder},
	"plugin":        {Glyph: "", Kind: KindFolder},
	"appex":         {Glyph: "", Kind: KindFolder},
	"rtfd":          {Glyph: "", Kind: KindDoc},
	"pages":         {Glyph: "", Kind: KindDoc},
	"numbers":       {Glyph: "", Kind: KindDoc},
	"key":           {Glyph: "", Kind: KindDoc},
	"sparsebundle":  {Glyph: "", Kind: KindArchive},
}

func init() {
	for _, k := range kinds {
		for _, e := range strings.Fields(k.exts) {
			if _, dup := byExt[e]; !dup {
				byExt[e] = k.icon
			}
		}
	}
}

// Attr says what else is known of an item besides its name.
type Attr struct {
	Dir  bool // a folder
	Open bool // a folder under the cursor
	Link bool // a symbolic link
	Exec bool // a file with an x permission
	Repo bool // a folder at the root of a Git repository
}

// For is the icon of a file or folder named name. open asks for an open
// folder's glyph, as for the folder under the cursor.
func For(name string, isDir, open bool) Icon {
	return Of(name, Attr{Dir: isDir, Open: open})
}

// Of is the icon of an item named name. A hidden item's (".name") is dim.
func Of(name string, at Attr) Icon {
	ic := of(strings.ToLower(name), at)
	ic.Dim = strings.HasPrefix(name, ".")
	return ic
}

func of(low string, at Attr) Icon {
	ext := ""
	if strings.LastIndex(low, ".") > 0 { // ".zshrc" has no extension
		ext = strings.TrimPrefix(path.Ext(low), ".")
	}
	switch {
	case at.Link && at.Dir:
		return LinkDir
	case at.Link:
		return LinkFile
	case at.Dir && at.Open:
		return DirOpen
	case at.Dir && at.Repo:
		return Repo
	case at.Dir:
		if ic, ok := byDir[low]; ok {
			return ic
		}
		if ic, ok := byDirExt[ext]; ok {
			return ic
		}
		return DirClosed
	}
	if ic, ok := byName[low]; ok {
		return ic
	}
	if ic, ok := byExt[ext]; ok {
		return ic
	}
	switch {
	case at.Exec:
		return Exec
	case strings.HasPrefix(low, "."):
		return Hidden
	}
	return File
}

// Fallback is the plain symbol shown when no Nerd Font is installed.
func Fallback(isDir, open bool) string {
	switch {
	case isDir && open:
		return "▾"
	case isDir:
		return "▸"
	}
	return " "
}

// Glyphs lists every glyph the package uses, for checking them against a
// font.
func Glyphs() []string {
	var out []string
	add := func(ic Icon) { out = append(out, ic.Glyph) }
	for _, ic := range []Icon{DirClosed, DirOpen, File, Hidden, Exec, LinkFile, LinkDir, Repo, App} {
		add(ic)
	}
	for _, m := range []map[string]Icon{byName, byExt, byDir, byDirExt} {
		for _, ic := range m {
			add(ic)
		}
	}
	return out
}
