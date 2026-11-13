package vfs

import (
	"path/filepath"
	"strings"
)

// packageExts are the folder kinds macOS shows as one file: apps, plug-ins,
// libraries, projects and document packages. Finder opens them; their
// insides are shown only on asking (显示包内容).
var packageExts = map[string]bool{}

func init() {
	for _, e := range []string{
		"app", "framework", "bundle", "plugin", "appex", "kext", "prefpane", "qlgenerator",
		"mdimporter", "saver", "xpc", "component",
		"photoslibrary", "musiclibrary", "imovielibrary", "fcpbundle", "logicx", "band",
		"xcodeproj", "xcworkspace", "playground", "xcarchive", "dsym", "docarchive",
		"rtfd", "scptd", "sparsebundle", "pages", "numbers", "key",
	} {
		packageExts[e] = true
	}
}

// IsPackageName reports whether a folder of this name is a package.
func IsPackageName(name string) bool {
	return packageExts[strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))]
}

// IsPackage reports whether e is a package folder on disk (not in an archive).
func IsPackage(e Entry) bool {
	return e.IsDir && IsPackageName(e.Name) && !InZip(e.Path)
}

// IsArchive reports whether e is a zip archive on disk, which opens with
// the system (Archive Utility extracts it) and is browsed only on asking.
func IsArchive(e Entry) bool {
	return !e.IsDir && IsZipName(e.Name) && !InZip(e.Path)
}

// Inside is the folder that shows what package or archive e holds.
func Inside(e Entry) string {
	if IsArchive(e) {
		return ZipRoot(e.Path)
	}
	return e.Path
}
