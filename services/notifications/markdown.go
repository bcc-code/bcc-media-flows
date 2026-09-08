package notifications

import "strings"

// Telegram is sent these messages with the legacy Markdown parse mode, whose
// parser recognises exactly four delimiters: `_`, `*`, `` ` `` and `[`. A
// dynamic value interpolated raw — a filename like TP01_2_2_TESTING.mov, an
// error string, a Vidispine ID — can open an entity that never closes, and
// Telegram then rejects the entire message with "Bad Request: can't parse
// entities" instead of dropping the formatting. That loses the whole
// notification, so every dynamic value in a RenderMarkdown implementation goes
// through one of the helpers below.

var legacyMarkdownEscaper = strings.NewReplacer(
	`_`, `\_`,
	`*`, `\*`,
	"`", "\\`",
	`[`, `\[`,
)

// escapeMarkdown makes s safe to interpolate into message text that sits
// outside any entity, where a backslash escape is honoured.
func escapeMarkdown(s string) string {
	return legacyMarkdownEscaper.Replace(s)
}

// escapeCode prepares s for the inside of a code span or code block. Backslash
// escapes are not honoured inside an entity, so a backtick can only be
// substituted: an apostrophe reads the same in a terminal-style block and
// cannot close the entity early.
func escapeCode(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}

// escapeCodeBlock prepares s for a ``` block, additionally normalising the
// CRLF line endings that Windows-side tooling reports and trimming the
// surrounding blank space that would otherwise pad the block.
func escapeCodeBlock(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(escapeCode(s))
}

// oneLine collapses every run of whitespace in s to a single space. Statuses
// and errors from other systems frequently carry a trailing CRLF, which breaks
// the sentence they are embedded in.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
