package plink

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestLinkFormModes(t *testing.T) {
	database, handler, cfg := lifecycleTestServer(t)
	link, err := database.CreateLink("taken-slug", "https://example.com/taken", "", "Tools")
	if err != nil {
		t.Fatal(err)
	}
	cookies := loginAs(t, handler, cfg)

	request := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		var body *strings.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		} else {
			body = strings.NewReader("")
		}
		req := httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
			if c.Name == "plink_csrf" {
				req.Header.Set("X-CSRF-Token", c.Value)
			}
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	newForm := request("GET", "/admin/links/new", nil).Body.String()
	if !strings.Contains(newForm, `hx-post="/admin/links"`) || strings.Contains(newForm, "hx-put=") {
		t.Fatalf("new link form should post to the collection: %s", newForm)
	}
	if !strings.Contains(newForm, "Tambah tautan") {
		t.Fatal("new link form should use the create title")
	}

	editForm := request("GET", "/admin/links/"+strconv.FormatInt(link.ID, 10)+"/edit", nil).Body.String()
	if !strings.Contains(editForm, `hx-put="/admin/links/`+strconv.FormatInt(link.ID, 10)+`"`) {
		t.Fatalf("edit link form should put to the link: %s", editForm)
	}
	if !strings.Contains(editForm, "Edit tautan") {
		t.Fatal("edit link form should use the edit title")
	}

	duplicate := request("POST", "/admin/links", url.Values{
		"slug": {"taken-slug"},
		"url":  {"https://example.com/duplicate"},
	}).Body.String()
	if !strings.Contains(duplicate, "slug already exists") {
		t.Fatalf("duplicate create should show the slug error: %s", duplicate)
	}
	if !strings.Contains(duplicate, `hx-post="/admin/links"`) || strings.Contains(duplicate, "hx-put=") {
		t.Fatalf("duplicate create must stay in create mode: %s", duplicate)
	}
	if !strings.Contains(duplicate, "Tambah tautan") || strings.Contains(duplicate, "Edit tautan") {
		t.Fatalf("duplicate create must keep the create title: %s", duplicate)
	}
}
