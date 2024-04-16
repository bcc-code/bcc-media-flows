package notifications

type Template interface {
	RenderHTML() (string, error)
	RenderMarkdown() (string, error)
}
