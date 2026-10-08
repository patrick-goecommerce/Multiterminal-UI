package backend

import "strings"

// toastSafe makes text safe to paste into go-toast's template. go-toast
// renders title, message and activation argument with text/template, unescaped,
// into an expandable PowerShell here-string (@" ... "@) and runs it. In such a
// string "$(...)" executes, "$var" expands and a backtick escapes; a line
// starting with "@ would end the here-string, and "]]>" would end the CDATA.
// Newlines are therefore collapsed first (so no line can start with "@), then
// backtick and dollar are escaped (backtick first, or its own escapes would be
// doubled) and "]]>" is split.
func toastSafe(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "$", "`$")
	return strings.ReplaceAll(s, "]]>", "] ]>")
}

// toastAttr is toastSafe for a value that sits inside an XML attribute of the
// same here-string (the launch argument).
func toastAttr(s string) string {
	return toastSafe(attrEscaper.Replace(s))
}

var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
