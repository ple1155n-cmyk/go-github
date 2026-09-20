package github

import (
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/go-querystring/query"
)

// ListOptions specifies the optional parameters to various List methods that
// support offset pagination.
type ListOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page int `url:"page,omitempty"`

	// For paginated result sets, the number of results to include per page.
	// A value of 0 means using the GitHub API default (typically 30 items).
	PerPage int `url:"per_page,omitempty"`
}

// Response is a GitHub API response. This wraps the standard http.Response
// and provides convenient access to pagination information.
type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of results.
	// Any or all of these may be set to zero if the response does not contain the
	// relevant Link header.
	NextPage  int
	PrevPage  int
	FirstPage int
	LastPage  int

	// PerPage is the number of items per page extracted from the Link header.
	// Defaults to 0 if the Link header omits the per_page parameter.
	PerPage int
}

// newResponse creates a new Response for the provided http.Response.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	parsePagination(response)
	return response
}

// parsePagination parses the RFC 5988 Link header and populates the pagination
// fields in Response, including NextPage, PrevPage, FirstPage, LastPage, and PerPage.
func parsePagination(resp *Response) {
	if resp == nil || resp.Response == nil {
		return
	}

	links, ok := resp.Response.Header["Link"]
	if !ok || len(links) == 0 {
		return
	}

	for _, linkHeader := range links {
		for _, part := range strings.Split(linkHeader, ",") {
			part = strings.TrimSpace(part)
			start := strings.IndexByte(part, '<')
			end := strings.IndexByte(part, '>')
			if start < 0 || end <= start {
				continue
			}

			linkURL := part[start+1 : end]
			u, err := url.Parse(linkURL)
			if err != nil {
				continue
			}

			q := u.Query()
			page, pageErr := strconv.Atoi(q.Get("page"))

			if pp, err := strconv.Atoi(q.Get("per_page")); err == nil && pp > 0 {
				resp.PerPage = pp
			}

			relIdx := strings.Index(part[end+1:], "rel=")
			if relIdx < 0 {
				continue
			}
			rel := strings.Trim(strings.TrimSpace(part[end+1+relIdx+4:]), `"'`)
			if semi := strings.IndexByte(rel, ';'); semi >= 0 {
				rel = strings.Trim(strings.TrimSpace(rel[:semi]), `"'`)
			}

			if pageErr == nil && page >= 1 {
				switch strings.ToLower(rel) {
				case "next":
					resp.NextPage = page
				case "prev":
					resp.PrevPage = page
				case "first":
					resp.FirstPage = page
				case "last":
					resp.LastPage = page
				}
			}
		}
	}
}

// ParsePagination parses HTTP Link response headers to populate pagination values.
func ParsePagination(resp *Response) {
	parsePagination(resp)
}

// addOptions adds the parameters in opts as URL query parameters to s. opts
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opts any) (string, error) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	if u.RawQuery != "" {
		existing := u.Query()
		for k, vals := range existing {
			if _, ok := qs[k]; !ok {
				qs[k] = vals
			}
		}
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

// AddOptions adds the parameters in opts as URL query parameters to s.
func AddOptions(s string, opts any) (string, error) {
	return addOptions(s, opts)
}
