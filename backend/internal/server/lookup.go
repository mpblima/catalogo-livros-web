package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ISBN lookup returns a draft only; saving remains an explicit user action.
type lookupClient struct {
	client              *http.Client
	open, google, links string
}

func isbnCode(raw string) (string, error) {
	n := regexp.MustCompile(`[^0-9Xx]`).ReplaceAllString(raw, "")
	n = strings.ToUpper(n)
	if len(n) == 18 {
		n = n[:13]
	}
	valid := false
	if len(n) == 10 {
		sum := 0
		valid = true
		for i, c := range n {
			v := int(c - '0')
			if c == 'X' && i == 9 {
				v = 10
			} else if c < '0' || c > '9' {
				valid = false
			}
			sum += (10 - i) * v
		}
		valid = valid && sum%11 == 0
		if valid {
			n = "978" + n[:9]
			sum = 0
			for i, c := range n {
				w := 1
				if i%2 == 1 {
					w = 3
				}
				sum += int(c-'0') * w
			}
			n += fmt.Sprint((10 - sum%10) % 10)
		}
	} else if len(n) == 13 && (strings.HasPrefix(n, "978") || strings.HasPrefix(n, "979")) {
		sum := 0
		valid = true
		for i, c := range n {
			if c < '0' || c > '9' {
				valid = false
			}
			w := 1
			if i%2 == 1 {
				w = 3
			}
			sum += int(c-'0') * w
		}
		valid = valid && sum%10 == 0
	}
	if !valid {
		return "", problem(400, "Informe um ISBN-10 ou ISBN-13 válido para pesquisar o livro.")
	}
	return n, nil
}
func (l lookupClient) data(ctx context.Context, endpoint string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CatalogoLivros/2.0 (personal ISBN lookup)")
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bibliographic HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2_000_001))
	if err != nil {
		return nil, err
	}
	if len(data) > 2_000_000 {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}
func (l lookupClient) json(ctx context.Context, endpoint string, target any) error {
	data, err := l.data(ctx, endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func textValue(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		if v > 0 {
			return fmt.Sprint(v)
		}
	case map[string]any:
		return textValue(v["value"])
	}
	return ""
}
func listNames(v any) string {
	out := []string{}
	if list, ok := v.([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				item = m["name"]
			}
			if s := textValue(item); s != "" {
				out = append(out, s)
			}
		}
	}
	return strings.Join(out, "; ")
}
func object(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

var tags = regexp.MustCompile(`(?s)<[^>]*>`)

func cleanText(v any) string {
	return strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(textValue(v), " "))), " ")
}
func httpsURL(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil {
		return ""
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	if u.Scheme != "https" {
		return ""
	}
	return u.String()
}
func (l lookupClient) find(ctx context.Context, raw string) (Book, error) {
	isbn, err := isbnCode(raw)
	if err != nil {
		return Book{}, err
	}
	b := Book{Kind: "physical", Format: "FISICO", Metadata: map[string]any{"isbn": isbn, "barcode": strings.TrimSpace(raw)}}
	var root map[string]any
	firstErr := l.json(ctx, l.open+"/api/books?bibkeys=ISBN:"+isbn+"&jscmd=data&format=json", &root)
	n := object(root["ISBN:"+isbn])
	if textValue(n["title"]) != "" {
		b.Title = textValue(n["title"])
		b.Summary = cleanText(n["notes"])
		for key, source := range map[string]string{"subtitle": "subtitle", "year": "publish_date", "pages": "number_of_pages"} {
			b.Metadata[key] = textValue(n[source])
		}
		for key, source := range map[string]string{"author": "authors", "publisher": "publishers", "subject": "subjects"} {
			b.Metadata[key] = listNames(n[source])
		}
		b.Metadata["keywords"] = b.Metadata["subject"]
		b.Metadata["source"] = "Open Library"
		cover := object(n["cover"])
		for _, size := range []string{"large", "medium", "small"} {
			if u := httpsURL(textValue(cover[size])); u != "" {
				b.CoverURL = u
				break
			}
		}
		if key := textValue(n["key"]); regexp.MustCompile(`^/books/OL[0-9]+M$`).MatchString(key) {
			var edition map[string]any
			if l.json(ctx, l.open+key+".json", &edition) == nil {
				b.Metadata["edition"] = textValue(edition["edition_name"])
				langs := []string{}
				if a, ok := edition["languages"].([]any); ok {
					for _, lang := range a {
						parts := strings.Split(textValue(object(lang)["key"]), "/")
						langs = append(langs, parts[len(parts)-1])
					}
				}
				b.Metadata["language"] = strings.Join(langs, "; ")
				if desc := cleanText(edition["description"]); desc != "" {
					b.Summary = desc
				} else if works, ok := edition["works"].([]any); ok && len(works) > 0 {
					key := textValue(object(works[0])["key"])
					if regexp.MustCompile(`^/works/OL[0-9]+W$`).MatchString(key) {
						var work map[string]any
						if l.json(ctx, l.open+key+".json", &work) == nil {
							b.Summary = cleanText(work["description"])
						}
					}
				}
			}
		}
	}
	// Google is a fallback and supplements missing metadata; never accept a nearby edition.
	var google map[string]any
	googleErr := l.json(ctx, l.google+"/volumes?q="+url.QueryEscape("isbn:"+isbn)+"&maxResults=5", &google)
	if items, ok := google["items"].([]any); ok {
		for _, item := range items {
			info := object(object(item)["volumeInfo"])
			exact := false
			if ids, ok := info["industryIdentifiers"].([]any); ok {
				for _, id := range ids {
					normalized, e := isbnCode(textValue(object(id)["identifier"]))
					if e == nil && normalized == isbn {
						exact = true
					}
				}
			}
			if !exact || textValue(info["title"]) == "" {
				continue
			}
			if b.Title == "" {
				b.Title = textValue(info["title"])
				b.Metadata["source"] = "Google Books"
			}
			if b.Summary == "" {
				b.Summary = cleanText(info["description"])
			}
			values := map[string]string{"subtitle": textValue(info["subtitle"]), "author": listNames(info["authors"]), "publisher": textValue(info["publisher"]), "year": textValue(info["publishedDate"]), "pages": textValue(info["pageCount"]), "language": textValue(info["language"]), "subject": listNames(info["categories"]), "keywords": listNames(info["categories"])}
			for k, v := range values {
				if textValue(b.Metadata[k]) == "" {
					b.Metadata[k] = v
				}
			}
			if b.CoverURL == "" {
				images := object(info["imageLinks"])
				for _, size := range []string{"thumbnail", "smallThumbnail"} {
					if u := httpsURL(textValue(images[size])); u != "" {
						b.CoverURL = u
						break
					}
				}
			}
			break
		}
	}
	if l.links != "" && (b.Title == "" || b.Summary == "" || textValue(b.Metadata["edition"]) == "") {
		if supplement, err := l.googlePage(ctx, isbn); err == nil {
			if b.Title == "" {
				b.Title = supplement.Title
			}
			if b.Summary == "" {
				b.Summary = supplement.Summary
			}
			if b.CoverURL == "" {
				b.CoverURL = supplement.CoverURL
			}
			for k, v := range supplement.Metadata {
				if textValue(b.Metadata[k]) == "" {
					b.Metadata[k] = v
				}
			}
		}
	}
	if b.Title == "" {
		if firstErr != nil || googleErr != nil {
			return b, problem(502, "Não foi possível consultar o livro agora. Preencha manualmente ou tente novamente.")
		}
		return b, problem(404, "Livro não encontrado para este ISBN. Você pode preencher os dados manualmente.")
	}
	if b.CoverURL == "" {
		candidate := "https://covers.openlibrary.org/b/isbn/" + isbn + "-M.jpg?default=false"
		req, _ := http.NewRequestWithContext(ctx, "GET", candidate, nil)
		resp, e := l.client.Do(req)
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				b.CoverURL = candidate
			}
		}
	}
	return b, nil
}
func (s *Server) lookupBook(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 28*time.Second)
	defer cancel()
	l := lookupClient{client: &http.Client{Timeout: 8 * time.Second}, open: "https://openlibrary.org", google: "https://www.googleapis.com/books/v1", links: "https://books.google.com/books"}
	b, err := l.find(ctx, r.URL.Query().Get("isbn"))
	if err != nil {
		return err
	}
	respond(w, 200, b)
	return nil
}

// The desktop also falls back to Google Books' bibliographic page when its JSON API is limited.
func (l lookupClient) googlePage(ctx context.Context, isbn string) (Book, error) {
	b := Book{Metadata: map[string]any{}}
	data, err := l.data(ctx, l.links+"?jscmd=viewapi&bibkeys=ISBN:"+isbn+"&callback=bookInfo")
	if err != nil {
		return b, err
	}
	raw := strings.TrimSpace(string(data))
	end := strings.LastIndex(raw, ")")
	if !strings.HasPrefix(raw, "bookInfo(") || end < 9 {
		return b, fmt.Errorf("invalid Google book response")
	}
	var root map[string]any
	if err = json.Unmarshal([]byte(raw[9:end]), &root); err != nil {
		return b, err
	}
	link := object(root["ISBN:"+isbn])
	if len(link) == 0 {
		link = object(object(root["books"])["ISBN:"+isbn])
	}
	if key := textValue(link["bib_key"]); key != "" && key != "ISBN:"+isbn {
		return b, fmt.Errorf("different ISBN")
	}
	b.CoverURL = httpsURL(textValue(link["thumbnail_url"]))
	address := httpsURL(textValue(link["info_url"]))
	u, err := url.Parse(address)
	if err != nil || u.Hostname() != "books.google.com" {
		return b, fmt.Errorf("invalid Google page URL")
	}
	q := u.Query()
	q.Set("hl", "en")
	u.RawQuery = q.Encode()
	data, err = l.data(ctx, u.String())
	if err != nil {
		return b, err
	}
	fields := map[string]string{}
	key := ""
	for _, cell := range regexp.MustCompile(`(?is)<td\b([^>]*)>(.*?)</td>`).FindAllStringSubmatch(string(data), -1) {
		class := htmlAttrs(cell[1])["class"]
		if strings.Contains(class, "metadata_label") {
			key = strings.ToLower(cleanText(cell[2]))
		} else if strings.Contains(class, "metadata_value") {
			fields[key] = cleanText(cell[2])
		}
	}
	for _, tag := range regexp.MustCompile(`(?is)<meta\b([^>]*)>`).FindAllStringSubmatch(string(data), -1) {
		attrs := htmlAttrs(tag[1])
		if strings.EqualFold(attrs["name"], "description") {
			b.Summary = cleanText(attrs["content"])
		}
	}
	pick := func(keys ...string) string {
		for _, key := range keys {
			if fields[key] != "" {
				return fields[key]
			}
		}
		return ""
	}
	b.Title = pick("title", "titulo")
	b.Metadata["author"] = pick("author", "authors")
	b.Metadata["publisher"] = pick("publisher", "editora")
	b.Metadata["edition"] = pick("edition", "edicao")
	b.Metadata["language"] = pick("language", "idioma")
	b.Metadata["subject"] = pick("subjects", "assuntos")
	b.Metadata["keywords"] = b.Metadata["subject"]
	b.Metadata["source"] = address
	if pages := regexp.MustCompile(`[0-9]+`).FindString(pick("length", "num. paginas")); pages != "" {
		b.Metadata["pages"] = pages
	}
	if year := regexp.MustCompile(`\b(?:19|20)[0-9]{2}\b`).FindString(textValue(b.Metadata["publisher"])); year != "" {
		b.Metadata["year"] = year
	}
	return b, nil
}
func htmlAttrs(raw string) map[string]string {
	out := map[string]string{}
	for _, a := range regexp.MustCompile(`(?is)([a-z_:]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`).FindAllStringSubmatch(raw, -1) {
		value := a[2]
		if value == "" {
			value = a[3]
		}
		out[strings.ToLower(a[1])] = html.UnescapeString(value)
	}
	return out
}
