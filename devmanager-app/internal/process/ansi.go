package process

import "regexp"

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07`)

// StripANSI replica strip_ansi de app/process/runner.py.
func StripANSI(text string) string {
	return ansiEscapeRe.ReplaceAllString(text, "")
}
