package renderer

type Renderer interface {
	Render(resumeJSON []byte) ([]byte, error)
}
