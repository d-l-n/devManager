package help

// CommunityLink es un enlace a canales de comunidad soporte. Con URL vacía
// (sin repo configurado) la UI lo oculta; el backend solo sirve el catálogo.
type CommunityLink struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Kind        string `json:"kind"` // github | discussions | issues | discord | website
}

// KnownCommunityLinks devuelve los enlaces oficiales del proyecto. El orden
// es el de la UI.
func KnownCommunityLinks() []CommunityLink {
	return []CommunityLink{
		{
			ID:          "repo", Label: "GitHub repository",
			Description: "Source code, releases and downloads.",
			URL:         "https://github.com/d-l-n/devManager", Kind: "github",
		},
		{
			ID:          "issues", Label: "Issue tracker",
			Description: "Report bugs and request features.",
			URL:         "https://github.com/d-l-n/devManager/issues", Kind: "issues",
		},
		{
			ID:          "discussions", Label: "Discussions",
			Description: "Questions, ideas and community Q&A.",
			URL:         "https://github.com/d-l-n/devManager/discussions", Kind: "discussions",
		},
		{
			ID:          "changelog-full", Label: "Full changelog",
			Description: "Complete version history on GitHub.",
			URL:         "https://github.com/d-l-n/devManager/blob/master/CHANGELOG.md", Kind: "website",
		},
	}
}
