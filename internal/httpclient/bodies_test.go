package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestURLEncodedBody(t *testing.T) {
	input := Request{Method: "POST", URL: "https://example.com", BodyType: BodyURLEncoded, Body: "{{unused_json}}", FormFields: []FormField{
		{Name: "name", Value: "{{name}}"}, {Name: "tag", Value: "one"}, {Name: "tag", Value: "two"}, {Name: "empty", Value: ""}, {Name: "multiline", Value: "a\nb=c"}, {Name: "disabled", Value: "{{missing}}", Disabled: true},
	}}
	resolved, err := input.Resolve(map[string]string{"name": "Ada & Grace + café"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := resolved.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Content-Type") != BodyURLEncoded || values.Get("name") != "Ada & Grace + café" || len(values["tag"]) != 2 || values.Get("multiline") != "a\nb=c" || req.ContentLength != int64(len(body)) {
		t.Fatalf("invalid encoded request: %s", body)
	}
	if input.FormFields[0].Value != "{{name}}" {
		t.Fatal("resolution changed template")
	}
}

func TestMultipartTextAndFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "my image.txt")
	contents := []byte("file contents\x00\xff")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	r := Request{Method: "POST", URL: "http://example.com", BodyType: BodyMultipart, FormFields: []FormField{
		{Name: "title", Value: "document"}, {Name: "title", Value: "second"}, {Name: "upload", Type: "file", FilePath: path}, {Name: "missing", Type: "file", FilePath: "missing", Disabled: true},
	}}
	req, err := r.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer req.Body.Close()
	kind, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || kind != BodyMultipart || params["boundary"] == "" {
		t.Fatal("missing multipart boundary")
	}
	reader := multipart.NewReader(req.Body, params["boundary"])
	var names []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, part.FormName())
		if part.FormName() == "upload" && (part.FileName() != "my image.txt" || !bytes.Equal(data, contents) || !strings.HasPrefix(part.Header.Get("Content-Type"), "text/plain")) {
			t.Fatal("file part changed")
		}
	}
	if strings.Join(names, ",") != "title,title,upload" {
		t.Fatalf("fields = %v", names)
	}
	replay, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	if _, err := io.Copy(io.Discard, replay); err != nil {
		t.Fatal(err)
	}
}

func TestMultipartErrorsAndCancellation(t *testing.T) {
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		req, err := (Request{Method: "POST", URL: "http://example.com", BodyType: BodyMultipart, FormFields: []FormField{{Name: "file", Type: "file", FilePath: path}}}).Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, req.Body); err == nil {
			t.Fatal("unreadable or non-regular file accepted")
		}
		req.Body.Close()
	}
	path := filepath.Join(t.TempDir(), "large.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := (Request{Method: "POST", URL: "http://example.com", BodyType: BodyMultipart, FormFields: []FormField{{Name: "file", Type: "file", FilePath: path}}}).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4096)
	if _, err := req.Body.Read(buffer); err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, req.Body); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled upload did not unblock")
	}
	req.Body.Close()
}

func TestFormValidation(t *testing.T) {
	for _, r := range []Request{
		{BodyType: BodyMultipart, FormFields: []FormField{{Name: "", Value: "value"}}},
		{BodyType: BodyMultipart, FormFields: []FormField{{Name: "bad\r\nfield", Value: "value"}}},
		{BodyType: BodyMultipart, FormFields: []FormField{{Name: "file", Type: "file"}}},
		{BodyType: BodyURLEncoded, FormFields: []FormField{{Name: "file", Type: "file", FilePath: "a.txt"}}},
		{BodyType: BodyMultipart, FormFields: []FormField{{Name: "field", Type: "unknown"}}},
		{BodyType: "text/plain"},
	} {
		r.Method = "POST"
		r.URL = "https://example.com"
		if _, err := r.Build(context.Background()); err == nil {
			t.Fatalf("accepted invalid body: %+v", r)
		}
	}
}

func TestUploadRedirectAndEcho(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	contents := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), 1<<20)...)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/307":
			http.Redirect(w, r, "/upload", http.StatusTemporaryRedirect)
			return
		case "/308":
			http.Redirect(w, r, "/upload", http.StatusPermanentRedirect)
			return
		}
		if err := r.ParseMultipartForm(32 << 10); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil || !bytes.Equal(data, contents) || header.Header.Get("Content-Type") != "image/png" || r.FormValue("description") != "hello upload" {
			t.Error("upload did not survive redirect")
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	for _, pathSuffix := range []string{"/307", "/308"} {
		template := Request{Method: "POST", URL: server.URL + pathSuffix, BodyType: BodyMultipart, FormFields: []FormField{{Name: "description", Value: "hello upload"}, {Name: "image", Type: "file", FilePath: path}}}
		req, err := template.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		client := *server.Client()
		var hops []RedirectHop
		ConfigureRedirects(&client, template, false, func(hop RedirectHop) { hops = append(hops, hop) })
		response, err := Execute(&client, req)
		if err != nil {
			t.Fatal(err)
		}
		if string(response.Body) != `{"ok":true}` || len(hops) != 1 || hops[0].Method != "POST" {
			t.Fatal("upload response or redirect hop missing")
		}
	}
}
