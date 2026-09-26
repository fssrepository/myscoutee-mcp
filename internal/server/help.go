package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"golang.org/x/net/html"
)

type Section struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text,omitempty"`
}
type Help struct {
	Source     string    `json:"source"`
	RevisionID string    `json:"revisionId"`
	Version    int       `json:"version"`
	Language   string    `json:"language"`
	Title      string    `json:"title"`
	Sections   []Section `json:"sections"`
}
type helpState struct {
	ActiveRevision *struct {
		ID       string `json:"id"`
		Version  int    `json:"version"`
		Lang     string `json:"lang"`
		Active   bool   `json:"active"`
		Title    string `json:"title"`
		Sections []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Blurb string `json:"blurb"`
			HTML  string `json:"contentHtml"`
		} `json:"sections"`
	} `json:"activeRevision"`
}

func ReadHelp(ctx context.Context, client *api.Client, lang, topic, query string, full bool) (Help, error) {
	if lang == "" {
		lang = "en"
	}
	if lang != "en" && lang != "hu" {
		return Help{}, errors.New("language must be en or hu")
	}
	if len(query) > 200 {
		return Help{}, errors.New("search is limited to 200 bytes")
	}
	var state helpState
	if err := client.GetHelp(ctx, lang, &state); err != nil {
		return Help{}, err
	}
	r := state.ActiveRevision
	if r == nil || !r.Active || r.Lang != lang || r.ID == "" || r.Version < 1 {
		return Help{}, errors.New("no active help revision for this language")
	}
	sourceBase := client.PublicBase
	if sourceBase == "" {
		sourceBase = client.Base
	}
	out := Help{Source: sourceBase + "/help/active?lang=" + lang, RevisionID: r.ID, Version: r.Version, Language: lang, Title: r.Title, Sections: []Section{}}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, s := range r.Sections {
		if topic != "" && topic != s.ID {
			continue
		}
		text := plainText(s.HTML)
		if q != "" && !strings.Contains(strings.ToLower(s.Title+" "+s.Blurb+" "+text), q) {
			continue
		}
		section := Section{ID: s.ID, Title: s.Title, Summary: s.Blurb}
		if full || topic != "" || q != "" {
			section.Text = text
		}
		out.Sections = append(out.Sections, section)
	}
	if topic != "" && len(out.Sections) == 0 {
		return Help{}, fmt.Errorf("unknown help topic; use list_help_topics first")
	}
	return out, nil
}
func plainText(content string) string {
	node, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return ""
	}
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "iframe":
				return
			case "li":
				out.WriteString("\n• ")
			case "p", "h1", "h2", "h3", "h4", "br":
				out.WriteByte('\n')
			}
		}
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.TrimSpace(out.String())
}
