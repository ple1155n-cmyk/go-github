package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name        string
		linkHeader  string
		wantNext    int
		wantPrev    int
		wantFirst   int
		wantLast    int
		wantPerPage int
	}{
		{
			name: "Both page and per_page present",
			linkHeader: `<https://api.github.com/user/repos?page=2&per_page=30>; rel="next", ` +
				`<https://api.github.com/user/repos?page=1&per_page=30>; rel="prev", ` +
				`<https://api.github.com/user/repos?page=1&per_page=30>; rel="first", ` +
				`<https://api.github.com/user/repos?page=5&per_page=30>; rel="last"`,
			wantNext:    2,
			wantPrev:    1,
			wantFirst:   1,
			wantLast:    5,
			wantPerPage: 30,
		},
		{
			name: "Only page present without per_page",
			linkHeader: `<https://api.github.com/user/repos?page=3>; rel="next", ` +
				`<https://api.github.com/user/repos?page=1>; rel="first"`,
			wantNext:    3,
			wantFirst:   1,
			wantPerPage: 0,
		},
		{
			name: "With extra query params such as sort and direction",
			linkHeader: `<https://api.github.com/user/repos?sort=created&direction=asc&page=4&per_page=25>; rel="next", ` +
				`<https://api.github.com/user/repos?sort=created&direction=asc&page=2&per_page=25>; rel="prev"`,
			wantNext:    4,
			wantPrev:    2,
			wantPerPage: 25,
		},
		{
			name:        "Empty link header",
			linkHeader:  "",
			wantNext:    0,
			wantPrev:    0,
			wantFirst:   0,
			wantLast:    0,
			wantPerPage: 0,
		},
		{
			name:        "Malformed link header",
			linkHeader:  `garbage-entry, <https://api.github.com/user/repos?page=invalid>; rel="next"`,
			wantNext:    0,
			wantPerPage: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpResp := &http.Response{Header: make(http.Header)}
			if tt.linkHeader != "" {
				httpResp.Header.Set("Link", tt.linkHeader)
			}
			resp := newResponse(httpResp)

			if resp.NextPage != tt.wantNext {
				t.Errorf("NextPage = %d, want %d", resp.NextPage, tt.wantNext)
			}
			if resp.PrevPage != tt.wantPrev {
				t.Errorf("PrevPage = %d, want %d", resp.PrevPage, tt.wantPrev)
			}
			if resp.FirstPage != tt.wantFirst {
				t.Errorf("FirstPage = %d, want %d", resp.FirstPage, tt.wantFirst)
			}
			if resp.LastPage != tt.wantLast {
				t.Errorf("LastPage = %d, want %d", resp.LastPage, tt.wantLast)
			}
			if resp.PerPage != tt.wantPerPage {
				t.Errorf("PerPage = %d, want %d", resp.PerPage, tt.wantPerPage)
			}
		})
	}
}

func TestAddOptions_PerPageZero(t *testing.T) {
	base := "https://api.github.com/resource"
	opts := ListOptions{Page: 1, PerPage: 0}

	got, err := addOptions(base, opts)
	if err != nil {
		t.Fatalf("addOptions error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}

	if _, exists := u.Query()["per_page"]; exists {
		t.Errorf("per_page parameter must not be emitted when PerPage is 0, got %q", got)
	}
	if u.Query().Get("page") != "1" {
		t.Errorf("page = %q, want %q", u.Query().Get("page"), "1")
	}
}

func TestAddOptions_PerPagePositive(t *testing.T) {
	base := "https://api.github.com/resource"
	opts := &ListOptions{Page: 2, PerPage: 50}

	got, err := addOptions(base, opts)
	if err != nil {
		t.Fatalf("addOptions error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}

	if u.Query().Get("per_page") != "50" {
		t.Errorf("per_page = %q, want 50", u.Query().Get("per_page"))
	}
	if u.Query().Get("page") != "2" {
		t.Errorf("page = %q, want 2", u.Query().Get("page"))
	}
}

func TestAddOptions_EmbeddedListOptions(t *testing.T) {
	type repoListOptions struct {
		ListOptions
		Visibility string `url:"visibility,omitempty"`
	}

	base := "https://api.github.com/user/repos"
	opts := repoListOptions{
		ListOptions: ListOptions{Page: 1, PerPage: 0},
		Visibility:  "public",
	}

	got, err := addOptions(base, opts)
	if err != nil {
		t.Fatalf("addOptions error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}

	if _, exists := u.Query()["per_page"]; exists {
		t.Errorf("per_page must not be emitted when embedded PerPage is 0, got %q", got)
	}
	if u.Query().Get("visibility") != "public" {
		t.Errorf("visibility = %q, want public", u.Query().Get("visibility"))
	}
}

func TestAddOptions_PreservesExistingQueryParams(t *testing.T) {
	base := "https://api.github.com/resource?sort=created&direction=desc"
	opts := ListOptions{Page: 2, PerPage: 0}

	got, err := addOptions(base, opts)
	if err != nil {
		t.Fatalf("addOptions error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}

	if u.Query().Get("sort") != "created" || u.Query().Get("direction") != "desc" {
		t.Errorf("existing query parameters were lost: %q", got)
	}
	if u.Query().Get("page") != "2" {
		t.Errorf("page = %q, want 2", u.Query().Get("page"))
	}
	if _, exists := u.Query()["per_page"]; exists {
		t.Errorf("per_page should not be set: %q", got)
	}
}

func TestPagination_ConsecutiveRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") == "0" {
			t.Errorf("server received invalid per_page=0")
		}

		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=2&per_page=30>; rel="next"`, r.URL.Path))
			w.WriteHeader(http.StatusOK)
		case "2":
			if r.URL.Query().Get("per_page") != "30" {
				t.Errorf("page 2 lost per_page context: %q", r.URL.RawQuery)
			}
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=1&per_page=30>; rel="prev"`, r.URL.Path))
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Initial request with PerPage: 0
	opts := ListOptions{Page: 1, PerPage: 0}
	initialURL, err := addOptions(server.URL+"/items", opts)
	if err != nil {
		t.Fatalf("addOptions: %v", err)
	}

	httpResp, err := http.Get(initialURL)
	if err != nil {
		t.Fatalf("GET page 1: %v", err)
	}
	resp := newResponse(httpResp)
	httpResp.Body.Close()

	if resp.NextPage != 2 || resp.PerPage != 30 {
		t.Fatalf("page 1 parse failed: NextPage=%d, PerPage=%d", resp.NextPage, resp.PerPage)
	}

	// Follow NextPage retaining server PerPage
	nextOpts := ListOptions{Page: resp.NextPage, PerPage: resp.PerPage}
	nextURL, err := addOptions(server.URL+"/items", nextOpts)
	if err != nil {
		t.Fatalf("addOptions: %v", err)
	}

	httpResp2, err := http.Get(nextURL)
	if err != nil {
		t.Fatalf("GET page 2: %v", err)
	}
	resp2 := newResponse(httpResp2)
	httpResp2.Body.Close()

	if resp2.PrevPage != 1 {
		t.Errorf("page 2 PrevPage = %d, want 1", resp2.PrevPage)
	}
}
