package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// A cover from a retailer is accepted only if the product page explicitly names
// the requested ISBN. Browsing stays on the configured retailer's own host.
func productCover(page, base, isbn string) string {
	if !regexp.MustCompile(`(?is)(?:isbn(?:-1[03])?|gtin1[34])[\s[:punct:]]{0,80}` + regexp.QuoteMeta(isbn) + `(?:[^0-9]|$)`).MatchString(page) {
		return ""
	}
	images := map[string]string{}
	for _, tag := range regexp.MustCompile(`(?is)<meta\b([^>]*)>`).FindAllStringSubmatch(page, -1) {
		a := htmlAttrs(tag[1])
		key := a["property"]
		if key == "" {
			key = a["name"]
		}
		images[strings.ToLower(key)] = a["content"]
	}
	origin, err := url.Parse(base)
	if err != nil {
		return ""
	}
	for _, key := range []string{"og:image:secure_url", "og:image", "twitter:image"} {
		if images[key] == "" {
			continue
		}
		u, e := url.Parse(images[key])
		if e != nil {
			continue
		}
		u = origin.ResolveReference(u)
		if u.Scheme == "https" && u.Hostname() != "" && u.User == nil {
			return u.String()
		}
	}
	return ""
}
func (l lookupClient) retailerCover(ctx context.Context, isbn, title string) string {
	shops := []string{"https://www.livrarialoyola.com.br/busca?busca=", "https://www.editoraluzdomundo.com.br/buscar?q="}
	if l.open != "https://openlibrary.org" {
		shops = []string{l.open + "/busca?busca="}
	}
	for _, shop := range shops {
		if ctx.Err() != nil {
			return ""
		}
		base, _ := url.Parse(shop)
		copyClient := *l.client
		copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > 3 || req.URL.Host != base.Host || req.URL.Scheme != base.Scheme {
				return fmt.Errorf("retailer redirect outside origin")
			}
			return nil
		}
		scoped := l
		scoped.client = &copyClient
		data, err := scoped.data(ctx, shop+isbn)
		if err != nil {
			continue
		}
		pages := []string{}
		seen := map[string]bool{}
		for _, tag := range regexp.MustCompile(`(?is)<a\b([^>]*)>`).FindAllStringSubmatch(string(data), -1) {
			href := htmlAttrs(tag[1])["href"]
			u, e := url.Parse(href)
			if e != nil || href == "" {
				continue
			}
			u = base.ResolveReference(u)
			if u.Host != base.Host || u.Scheme != base.Scheme || u.User != nil || seen[u.String()] {
				continue
			}
			path := strings.ToLower(u.Path)
			if !strings.Contains(path, "/produto/") && !strings.Contains(path, "/livro/") {
				continue
			}
			seen[u.String()] = true
			pages = append(pages, u.String())
			if len(pages) == 5 {
				break
			}
		}
		for _, page := range pages {
			data, e := scoped.data(ctx, page)
			if e == nil {
				if cover := productCover(string(data), page, isbn); cover != "" {
					return cover
				}
			}
		}
	}
	return ""
}
