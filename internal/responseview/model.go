// Package responseview prepares bounded, read-only representations of HTTP bodies.
// It has no dependency on Fyne. Body always aliases the original retained payload.
package responseview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hsm-gustavo/relay/internal/httpclient"
)

const (
	TextLimit      = 256 << 10
	TreeLimit      = 256 << 10
	MaxNodes       = 5000
	MaxDepth       = 64
	MaxImageBytes  = 8 << 20
	MaxImagePixels = 16_000_000
)
const (
	JSON   = "json"
	Text   = "text"
	Binary = "binary"
	Image  = "image"
	Empty  = "empty"
)
const (
	Pretty = "Pretty"
	Raw    = "Raw"
	Tree   = "Tree"
)

type Model struct {
	HeaderPreview  string
	HeaderLimited  bool
	Detected       bool
	Body           []byte
	Kind           string
	ContentType    string
	Note           string
	Incomplete     bool
	RawText        string
	PrettyText     string
	TextLimited    bool
	PrettyLimited  bool
	SupportsPretty bool
	Nodes          map[string]Node
	TreeNote       string
	Image          image.Image
}

func CanPrepareInline(r *httpclient.Response) bool {
	if r == nil {
		return true
	}
	bytes := len(r.Body)
	for key, values := range r.Headers {
		for _, value := range values {
			bytes += len(key) + len(value) + 4
			if bytes > 8<<10 {
				return false
			}
		}
		if bytes > 8<<10 {
			return false
		}
	}
	return len(r.Body) <= 8<<10 && !strings.HasPrefix(r.Headers.Get("Content-Type"), "image/") && !strings.HasPrefix(http.DetectContentType(r.Body), "image/")
}

func mediaType(r *httpclient.Response) string {
	header := r.Headers.Get("Content-Type")
	if header != "" && len(header) <= 1024 {
		if kind, _, err := mime.ParseMediaType(header); err == nil {
			return strings.ToLower(kind)
		}
	}
	return strings.Split(http.DetectContentType(r.Body), ";")[0]
}

func textual(body []byte) bool {
	for len(body) > 0 {
		r, size := utf8.DecodeRune(body)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
		body = body[size:]
	}
	return true
}

func Prepare(r *httpclient.Response) *Model {
	if r == nil {
		return nil
	}
	m := &Model{Body: r.Body, Incomplete: r.Incomplete, ContentType: mediaType(r), Detected: r.Headers.Get("Content-Type") == ""}
	headerPage := Window(httpclient.FormatHeaders(r.Headers), 0)
	m.HeaderPreview = headerPage.Text
	m.HeaderLimited = headerPage.More
	if r.Incomplete {
		m.Note = "Incomplete capture: only retained bytes can be copied or saved; this may not be a complete file."
	}
	if len(r.Body) == 0 {
		m.Kind = Empty
		return m
	}
	ct := m.ContentType
	declaredJSON := ct == "application/json" || ct == "text/json" || strings.HasSuffix(ct, "+json")
	// Preserve prior JSON detection when a server supplied no Content-Type.
	jsonBody := declaredJSON || (r.Headers.Get("Content-Type") == "" && json.Valid(r.Body))
	if jsonBody && json.Valid(r.Body) && utf8.Valid(r.Body) {
		m.Kind = JSON
		if r.Headers.Get("Content-Type") == "" {
			m.ContentType = "application/json"
		}
	} else if strings.HasPrefix(ct, "image/") && ct != "image/svg+xml" {
		m.Kind = Image
		m.prepareImage()
		return m
	} else if strings.HasPrefix(ct, "text/") || ct == "application/xml" || strings.HasSuffix(ct, "+xml") || ct == "application/javascript" || ct == "application/x-www-form-urlencoded" || declaredJSON {
		if textual(r.Body) {
			m.Kind = Text
		} else {
			m.Kind = Binary
			m.addNote("Text cannot be displayed safely as UTF-8; save the original bytes instead.")
		}
		if declaredJSON {
			m.addNote("Invalid JSON; displaying the retained content without JSON formatting.")
		}
	} else {
		m.Kind = Binary
	}
	if m.Kind == Binary {
		return m
	}
	sample := r.Body
	if len(sample) > TextLimit {
		sample = sample[:TextLimit]
		for len(sample) > 0 && !utf8.Valid(sample) {
			sample = sample[:len(sample)-1]
		}
		m.TextLimited = true
		m.addNote("Text navigation and search are limited to the first 256 KiB. Copy and Save use all retained bytes.")
	}
	m.RawText = string(sample)
	m.PrettyText = m.RawText
	if m.Kind == Text && (ct == "application/xml" || ct == "text/xml" || strings.HasSuffix(ct, "+xml")) {
		formatted, note := FormatXML(r.Body)
		if note != "" {
			m.addNote(note)
		} else {
			m.PrettyText = formatted
			m.SupportsPretty = true
		}
	}
	if m.Kind == JSON {
		m.SupportsPretty = true
		if httpclient.CanFormatJSON(r.Body) {
			m.PrettyText = httpclient.FormatBody(r.Body, true)
		} else {
			m.PrettyLimited = true
			m.addNote("Pretty formatting limit reached; displaying raw text instead.")
		}
		if !r.Incomplete {
			m.Nodes, m.TreeNote = BuildTree(r.Body)
		} else {
			m.TreeNote = "Tree unavailable for an incomplete capture."
		}
	}
	return m
}

func (m *Model) addNote(note string) {
	if m.Note != "" {
		m.Note += "\n"
	}
	m.Note += note
}
func (m *Model) TextFor(mode string) string {
	if mode == Pretty {
		return m.PrettyText
	}
	return m.RawText
}
func (m *Model) CopyText(mode string) (string, error) {
	if m.Kind == Binary || m.Kind == Image {
		return "", fmt.Errorf("this response is not textual; save its bytes instead")
	}
	if m.SupportsPretty && mode == Pretty && !m.PrettyLimited {
		return m.PrettyText, nil
	}
	return string(m.Body), nil
}
func (m *Model) prepareImage() {
	if m.Incomplete {
		m.addNote("Image preview unavailable for an incomplete capture.")
		return
	}
	if len(m.Body) > MaxImageBytes {
		m.addNote("Image preview limited to 8 MiB. Save the original bytes to inspect this image.")
		return
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(m.Body))
	if err != nil {
		m.addNote("Unsupported or invalid image; save its original bytes to inspect it.")
		return
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > MaxImagePixels/int64(config.Height) {
		m.addNote("Image preview limited to 16 million pixels. Save the original bytes instead.")
		return
	}
	m.Image, _, err = image.Decode(bytes.NewReader(m.Body))
	if err != nil {
		m.Image = nil
		m.addNote("Image decoding failed; the original retained bytes remain available to save.")
	}
}

func (m *Model) Filename() string {
	ext := ".bin"
	switch {
	case m.Kind == JSON || strings.HasSuffix(m.ContentType, "+json") || m.ContentType == "application/json":
		ext = ".json"
	case m.ContentType == "text/html":
		ext = ".html"
	case m.ContentType == "image/svg+xml":
		ext = ".svg"
	case m.ContentType == "application/xml" || m.ContentType == "text/xml" || strings.HasSuffix(m.ContentType, "+xml"):
		ext = ".xml"
	case m.ContentType == "image/png":
		ext = ".png"
	case m.ContentType == "image/jpeg":
		ext = ".jpg"
	case m.ContentType == "image/gif":
		ext = ".gif"
	case m.ContentType == "image/webp":
		ext = ".webp"
	case m.ContentType == "application/pdf":
		ext = ".pdf"
	case m.ContentType == "application/zip":
		ext = ".zip"
	case m.Kind == Text:
		ext = ".txt"
	}
	if m.Incomplete {
		return "response.partial" + ext
	}
	return "response" + ext
}

// WriteBody saves retained bytes, never a Pretty/Tree representation.
func WriteBody(writer io.Writer, body []byte) error {
	_, err := io.Copy(writer, bytes.NewReader(body))
	return err
}

var workers = make(chan struct{}, 2)

// PrepareContext bounds concurrent expensive decodes and drops queued stale work.
func PrepareContext(ctx context.Context, r *httpclient.Response) *Model {
	select {
	case workers <- struct{}{}:
	case <-ctx.Done():
		return nil
	}
	defer func() { <-workers }()
	if ctx.Err() != nil {
		return nil
	}
	model := Prepare(r)
	if ctx.Err() != nil {
		return nil
	}
	return model
}
