package responseview

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

func response(contentType, body string) *httpclient.Response {
	return &httpclient.Response{Status: "200 OK", Headers: http.Header{"Content-Type": {contentType}}, Body: []byte(body)}
}

func TestClassificationAndOriginalBytes(t *testing.T) {
	for _, tt := range []struct{ ct, body, kind string }{
		{"application/json", `{"id":42}`, JSON}, {"application/problem+json; charset=utf-8", `{"error":true}`, JSON},
		{"application/ld+json", `[1,2]`, JSON}, {"", `{"ok":true}`, JSON},
		{"application/json", `{"broken":}`, Text}, {"text/plain", "hello\nworld", Text}, {"text/html", "<script>alert('x')</script>", Text},
		{"application/xml", "<root><item>text</item></root>", Text}, {"image/svg+xml", "<svg></svg>", Text},
		{"application/octet-stream", "text but explicitly binary", Binary}, {"application/pdf", "%PDF-binary", Binary}, {"application/zip", "PK\x00\x01", Binary},
		{"text/plain", "\xff\x00", Binary}, {"application/json", "{\"value\":\"\xff\"}", Binary}, {"application/json", "", Empty},
	} {
		t.Run(tt.ct+tt.body, func(t *testing.T) {
			r := response(tt.ct, tt.body)
			original := append([]byte(nil), r.Body...)
			model := Prepare(r)
			if model.Kind != tt.kind {
				t.Fatalf("kind = %s, want %s (%s)", model.Kind, tt.kind, model.Note)
			}
			if !bytes.Equal(r.Body, original) || !bytes.Equal(model.Body, original) {
				t.Fatal("preparation changed original bytes")
			}
			if tt.ct == "application/json" && tt.kind == Text && !strings.Contains(model.Note, "Invalid JSON") {
				t.Fatal("invalid JSON was not explained")
			}
		})
	}
}

func TestPrettyTreeAndNumberPrecision(t *testing.T) {
	body := `{"user":{"id":9007199254740993,"name":"Ada","active":true,"nothing":null},"roles":["admin","editor"],"number":1.20e+30,"same":1,"same":2}`
	model := Prepare(response("application/json", body))
	if !strings.Contains(model.PrettyText, "\n  \"user\"") || !strings.Contains(model.PrettyText, "9007199254740993") || !strings.Contains(model.PrettyText, "1.20e+30") {
		t.Fatal("Pretty lost indentation or number spelling")
	}
	if model.TreeNote != "" || len(model.Nodes["0"].Children) != 5 {
		t.Fatal("tree lost properties or duplicate keys")
	}
	var numberFound, arrayFound, nullFound bool
	for _, node := range model.Nodes {
		if node.Name == "id" && node.Value == "9007199254740993" {
			numberFound = true
		}
		if node.Name == "[1]" && node.Value == `"editor"` {
			arrayFound = true
		}
		if node.Kind == "null" && node.Value == "null" {
			nullFound = true
		}
	}
	if !numberFound || !arrayFound || !nullFound {
		t.Fatal("tree did not preserve types, indices or numbers")
	}
	raw, _ := model.CopyText(Raw)
	pretty, _ := model.CopyText(Pretty)
	if raw != body || pretty != model.PrettyText {
		t.Fatal("copy mode changed source or returned wrong representation")
	}
}

func TestLargeAndDeepLimits(t *testing.T) {
	large := `{"text":"` + strings.Repeat("x", TextLimit+100) + `"}`
	model := Prepare(response("application/json", large))
	if len(model.RawText) > TextLimit || !model.TextLimited || !model.PrettyLimited || model.Nodes != nil || model.TreeNote == "" {
		t.Fatal("large JSON was not bounded")
	}
	copy, _ := model.CopyText(Pretty)
	if copy != large {
		t.Fatal("large-body copy did not retain the payload")
	}
	deep := strings.Repeat("[", MaxDepth+2) + "0" + strings.Repeat("]", MaxDepth+2)
	if nodes, note := BuildTree([]byte(deep)); nodes != nil || note == "" {
		t.Fatal("deep tree was not bounded")
	}
	many := "[" + strings.Repeat("0,", MaxNodes) + "0]"
	if nodes, note := BuildTree([]byte(many)); nodes != nil || note == "" {
		t.Fatal("node limit was not applied")
	}
	r := response("application/json", `{"ok":true}`)
	r.Incomplete = true
	partial := Prepare(r)
	if partial.Nodes != nil || !strings.Contains(partial.Filename(), ".partial.") || !strings.Contains(partial.Note, "Incomplete") {
		t.Fatal("incomplete capture was presented as complete")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if PrepareContext(ctx, r) != nil {
		t.Fatal("cancelled preparation returned a result")
	}
}

func TestImagePreviewAndBinaryFallback(t *testing.T) {
	imageData := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	imageData.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageData); err != nil {
		t.Fatal(err)
	}
	r := response("image/png", "")
	r.Body = encoded.Bytes()
	model := Prepare(r)
	if model.Kind != Image || model.Image == nil || model.Image.Bounds().Dx() != 32 || model.Filename() != "response.png" {
		t.Fatal("PNG preview failed")
	}
	if _, err := model.CopyText(Raw); err == nil {
		t.Fatal("binary image should use Save, not text clipboard")
	}
	oversized := append([]byte(nil), r.Body...)
	binary.BigEndian.PutUint32(oversized[16:20], 5000)
	binary.BigEndian.PutUint32(oversized[20:24], 5000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if result := Prepare(&httpclient.Response{Headers: r.Headers, Body: oversized}); result.Image != nil || !strings.Contains(result.Note, "pixels") {
		t.Fatal("image pixel budget was not applied before decoding")
	}
	r.Incomplete = true
	if model := Prepare(r); model.Image != nil || model.Note == "" {
		t.Fatal("partial image was decoded")
	}
	invalid := Prepare(response("image/jpeg", "not jpeg"))
	if invalid.Image != nil || invalid.Note == "" || invalid.Filename() != "response.jpg" {
		t.Fatal("invalid image did not fall back safely")
	}
	huge := Prepare(response("image/png", strings.Repeat("x", MaxImageBytes+1)))
	if huge.Image != nil || !strings.Contains(huge.Note, "8 MiB") {
		t.Fatal("image size limit was not applied")
	}
}

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

func TestSaveOriginalBytesAndErrors(t *testing.T) {
	for _, body := range [][]byte{[]byte("{\"id\":42}\r\n"), {0, 255, 1, 10}, nil} {
		var saved bytes.Buffer
		if err := WriteBody(&saved, body); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved.Bytes(), body) {
			t.Fatal("saved bytes differ from original")
		}
	}
	if err := WriteBody(badWriter{}, []byte("x")); err == nil {
		t.Fatal("write error was lost")
	}
	if err := WriteBody(shortWriter{}, []byte("x")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
}

func TestXMLFormattingDoesNotLoadExternalResources(t *testing.T) {
	r := response("application/xml", "<root><item>hello</item></root>")
	model := Prepare(r)
	if !model.SupportsPretty || !strings.Contains(model.PrettyText, "\n  <item>") {
		t.Fatal("XML formatting unavailable")
	}
	if string(r.Body) != "<root><item>hello</item></root>" {
		t.Fatal("XML source changed")
	}
	for _, source := range []string{`<!DOCTYPE root SYSTEM "http://example.com/external"><root/>`, `<root>`, `<root/>trailing`, `<root xmlns="urn:example"><item/></root>`} {
		if formatted, note := FormatXML([]byte(source)); formatted != "" || note == "" {
			t.Fatal("invalid or directive XML was formatted")
		}
	}
}
