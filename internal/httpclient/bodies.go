package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

const (
	BodyJSON       = "application/json"
	BodyURLEncoded = "application/x-www-form-urlencoded"
	BodyMultipart  = "multipart/form-data"
)

type FormField struct {
	ID       string `json:",omitempty"`
	Name     string
	Value    string
	Type     string `json:",omitempty"` // Empty or text; file is supported only in multipart.
	FilePath string `json:",omitempty"`
	Disabled bool   `json:",omitempty"`
}

func (r Request) EffectiveBodyType() string {
	if r.BodyType == "" {
		return BodyJSON
	}
	return r.BodyType
}

// body constructs only metadata on the UI thread. Multipart file I/O starts
// lazily when the transport reads, and never buffers the complete upload.
func (r Request) body(ctx context.Context) (io.ReadCloser, string, func() (io.ReadCloser, error), error) {
	switch r.EffectiveBodyType() {
	case BodyJSON:
		if strings.TrimSpace(r.Body) == "" {
			return nil, "", nil, nil
		}
		if !json.Valid([]byte(r.Body)) {
			return nil, "", nil, fmt.Errorf("request body must be valid JSON")
		}
		get := func() (io.ReadCloser, error) { return stringBody{strings.NewReader(r.Body)}, nil }
		reader, _ := get()
		return reader, BodyJSON, get, nil
	case BodyURLEncoded, BodyMultipart:
		fields := append([]FormField(nil), r.FormFields...)
		values := make(url.Values)
		for i, field := range fields {
			if field.Disabled {
				continue
			}
			if strings.TrimSpace(field.Name) == "" || strings.IndexFunc(field.Name, unicode.IsControl) >= 0 {
				return nil, "", nil, fmt.Errorf("form field %d needs a name without line breaks", i+1)
			}
			switch field.Type {
			case "", "text":
				values.Add(field.Name, field.Value)
			case "file":
				if r.EffectiveBodyType() != BodyMultipart {
					return nil, "", nil, fmt.Errorf("file fields require multipart/form-data")
				}
				if strings.TrimSpace(field.FilePath) == "" {
					return nil, "", nil, fmt.Errorf("select a file for form field %q", field.Name)
				}
			default:
				return nil, "", nil, fmt.Errorf("unknown form field type %q", field.Type)
			}
		}
		if r.EffectiveBodyType() == BodyURLEncoded {
			encoded := values.Encode()
			get := func() (io.ReadCloser, error) { return stringBody{strings.NewReader(encoded)}, nil }
			reader, _ := get()
			return reader, BodyURLEncoded, get, nil
		}
		// Reuse the boundary when replaying a 307/308 redirect.
		boundary := multipart.NewWriter(io.Discard).Boundary()
		get := func() (io.ReadCloser, error) { return newMultipartStream(ctx, fields, boundary), nil }
		reader, _ := get()
		return reader, BodyMultipart + "; boundary=" + boundary, get, nil
	default:
		return nil, "", nil, fmt.Errorf("choose JSON, URL-encoded, or multipart body type")
	}
}

type multipartStream struct {
	reader   *io.PipeReader
	writer   *io.PipeWriter
	once     sync.Once
	stop     func() bool
	ctx      context.Context
	fields   []FormField
	boundary string
}

func newMultipartStream(ctx context.Context, fields []FormField, boundary string) *multipartStream {
	reader, writer := io.Pipe()
	stream := &multipartStream{reader: reader, writer: writer, ctx: ctx, fields: fields, boundary: boundary}
	stream.stop = context.AfterFunc(ctx, func() { _ = reader.CloseWithError(ctx.Err()) })
	return stream
}
func (s *multipartStream) Read(p []byte) (int, error) {
	s.once.Do(func() { go s.write() })
	n, err := s.reader.Read(p)
	if err != nil && s.ctx.Err() != nil {
		return n, s.ctx.Err()
	}
	return n, err
}
func (s *multipartStream) Close() error { s.stop(); return s.reader.Close() }
func (s *multipartStream) write() {
	defer s.stop()
	writer := multipart.NewWriter(s.writer)
	if err := writer.SetBoundary(s.boundary); err != nil {
		s.writer.CloseWithError(err)
		return
	}
	for _, field := range s.fields {
		if field.Disabled {
			continue
		}
		if err := s.ctx.Err(); err != nil {
			s.writer.CloseWithError(err)
			return
		}
		var err error
		if field.Type == "file" {
			err = writeFilePart(s.ctx, writer, field)
		} else {
			err = writer.WriteField(field.Name, field.Value)
		}
		if err != nil {
			s.writer.CloseWithError(err)
			return
		}
	}
	if err := writer.Close(); err != nil {
		s.writer.CloseWithError(err)
		return
	}
	s.writer.Close()
}

func writeFilePart(ctx context.Context, writer *multipart.Writer, field FormField) error {
	info, err := os.Stat(field.FilePath)
	if err != nil {
		return fmt.Errorf("open upload for %q: %w", field.Name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("upload for %q must be a regular file", field.Name)
	}
	file, err := os.Open(field.FilePath)
	if err != nil {
		return fmt.Errorf("open upload for %q: %w", field.Name, err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("upload for %q must be a regular file", field.Name)
	}
	filename := filepath.Base(field.FilePath)
	if strings.IndexFunc(filename, unicode.IsControl) >= 0 {
		return fmt.Errorf("upload filename cannot contain line breaks")
	}
	sample := make([]byte, 512)
	n, err := file.Read(sample)
	if err != nil && err != io.EOF {
		return err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if contentType == "" {
		contentType = http.DetectContentType(sample[:n])
	}
	header := make(textproto.MIMEHeader)
	escape := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	header.Set("Content-Disposition", `form-data; name="`+escape.Replace(field.Name)+`"; filename="`+escape.Replace(filename)+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err = part.Write(sample[:n]); err != nil {
		return err
	}
	_, err = io.Copy(part, contextReader{ctx: ctx, reader: file})
	if err != nil {
		return fmt.Errorf("read upload for %q: %w", field.Name, err)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type stringBody struct{ *strings.Reader }

func (stringBody) Close() error { return nil }
