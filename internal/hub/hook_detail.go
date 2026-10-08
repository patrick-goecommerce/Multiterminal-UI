package hub

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// What a hook event says the agent wants, as one line for the waiting list.

// sentenceEnd is the end of a sentence before the question: a full stop,
// exclamation or question mark followed by space.
var sentenceEnd = regexp.MustCompile(`[.!?…]["'»“”)\]*_~]*\s+`)

// paragraphs splits a message into its non-empty paragraphs, code blocks
// removed. mtui-hook sends only the tail of a long message, which can start
// inside a code block: an odd number of fences means the first one closes a
// block whose start was cut off.
func paragraphs(msg string) []string {
	if strings.Count(msg, "```")%2 == 1 {
		msg = msg[strings.Index(msg, "```")+3:]
	}
	msg = codeFence.ReplaceAllString(msg, "")
	var paras []string
	for _, p := range strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(p) != "" {
			paras = append(paras, p)
		}
	}
	return paras
}

// questionSentence returns the last question in a paragraph, from the start
// of its sentence to its question mark, or "" when there is none.
func questionSentence(para string) string {
	locs := sentenceQuestion.FindAllStringIndex(para, -1)
	if len(locs) == 0 {
		return ""
	}
	qStart := locs[len(locs)-1][0]
	_, size := utf8.DecodeRuneInString(para[qStart:])
	start := 0
	for _, m := range sentenceEnd.FindAllStringIndex(para[:qStart], -1) {
		start = m[1]
	}
	return strings.TrimLeft(strings.TrimSpace(para[start:qStart+size]), "*_ ")
}

// stopDetail is what a Stop message shows: the question when the message
// ends by asking one, otherwise its last paragraph.
func stopDetail(msg string) string {
	paras := paragraphs(msg)
	if len(paras) == 0 {
		return ""
	}
	last := paras[len(paras)-1]
	if isList(last) && len(paras) > 1 && sentenceQuestion.MatchString(paras[len(paras)-2]) {
		last = paras[len(paras)-2]
	}
	if q := questionSentence(last); q != "" {
		return clipDetail(q)
	}
	return clipDetail(strings.TrimLeft(last, "*_ "))
}

// hookDetail is what one hook event says the agent wants.
func hookDetail(ev hooks.Event) string {
	switch {
	case ev.Event == "Stop":
		return stopDetail(ev.Message)
	case ev.Detail != "":
		return clipDetail(ev.Detail)
	case ev.Event == "Notification":
		return clipDetail(ev.Message)
	}
	return ""
}
