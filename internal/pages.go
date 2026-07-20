package internal

import (
	"os"
	"strings"
)

type Renderer struct {
	header string
	footer string
	vars   map[string]string
}

func NewRenderer(cfg *Config) (*Renderer, error) {
	header, err := os.ReadFile("header.html")
	if err != nil {
		return nil, err
	}
	footer, err := os.ReadFile("footer.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{
		header: string(header),
		footer: string(footer),
		vars: map[string]string{
			"title":          cfg.SiteName,
			"site_name":      cfg.SiteName,
			"copyright_name": cfg.CopyrightName,
		},
	}, nil
}

func (r *Renderer) Render(components []string) string {
	parts := make([]string, 0, len(components)+2)
	parts = append(parts, r.header)
	parts = append(parts, components...)
	parts = append(parts, r.footer)
	page := strings.Join(parts, "\n\n")
	for k, v := range r.vars {
		page = strings.ReplaceAll(page, "{{"+k+"}}", v)
	}
	return page
}

func (r *Renderer) RenderWrite(components []string, dest string) error {
	return os.WriteFile(dest, []byte(r.Render(components)), 0o644)
}
