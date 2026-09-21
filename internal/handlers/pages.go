package handlers

import (
	"fmt"
	"net/http"
)

// AboutHandler serves GET /about.
func (h *Handler) AboutHandler(w http.ResponseWriter, r *http.Request) {
	h.render(w, "about", nil)
}

// ContactHandler serves GET /contact.
func (h *Handler) ContactHandler(w http.ResponseWriter, r *http.Request) {
	h.render(w, "contact", nil)
}

// RobotsHandler serves GET /robots.txt.
func (h *Handler) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /polls/\n\nSitemap: %s/sitemap.xml\n", baseURL(r))
}

// SitemapHandler serves GET /sitemap.xml with the site's public, indexable pages.
// Poll pages are excluded — they're unlisted/shareable, not meant to be crawled.
func (h *Handler) SitemapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	base := baseURL(r)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/</loc></url>
  <url><loc>%s/about</loc></url>
  <url><loc>%s/contact</loc></url>
</urlset>
`, base, base, base)
}

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
