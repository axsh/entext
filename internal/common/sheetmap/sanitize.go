package sheetmap

import "regexp"

var (
	forbiddenChars  = regexp.MustCompile(`[\\/:*?"<>|]`)
	spaceChars      = regexp.MustCompile(`\s+`)
	multiUnderscore = regexp.MustCompile(`_+`)
)

func SanitizeFilename(name string) string {
	name = spaceChars.ReplaceAllString(name, "_")
	name = forbiddenChars.ReplaceAllString(name, "")
	name = multiUnderscore.ReplaceAllString(name, "_")
	return trimUnderscore(name)
}

func trimUnderscore(name string) string {
	for len(name) > 0 && name[0] == '_' {
		name = name[1:]
	}
	for len(name) > 0 && name[len(name)-1] == '_' {
		name = name[:len(name)-1]
	}
	return name
}
