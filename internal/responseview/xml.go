package responseview

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > TextLimit {
		return 0, fmt.Errorf("XML formatting limit reached")
	}
	return b.Buffer.Write(p)
}

// FormatXML uses only encoding/xml tokens, never external entities or resources.
func FormatXML(body []byte) (string, string) {
	if len(body) > 64<<10 {
		return "", "XML formatting limited to 64 KiB; displaying raw source."
	}
	var output limitedBuffer
	decoder := xml.NewDecoder(bytes.NewReader(body))
	encoder := xml.NewEncoder(&output)
	encoder.Indent("", "  ")
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "Invalid XML; displaying raw source."
		}
		switch value := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return "", "Invalid XML; displaying raw source."
			}
		case xml.StartElement:
			if value.Name.Space != "" {
				return "", "Namespaced XML is shown as raw source to preserve its original representation."
			}
			for _, attribute := range value.Attr {
				if attribute.Name.Space != "" || attribute.Name.Local == "xmlns" {
					return "", "Namespaced XML is shown as raw source to preserve its original representation."
				}
			}
			if depth == 0 {
				roots++
			}
			depth++
			if depth > MaxDepth {
				return "", "XML nesting limit reached; displaying raw source."
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return "", "XML with directives is displayed as raw source; no external resources are loaded."
		}
		if err := encoder.EncodeToken(token); err != nil {
			return "", "XML formatting limit reached; displaying raw source."
		}
	}
	if roots != 1 || depth != 0 {
		return "", "Invalid XML; displaying raw source."
	}
	if err := encoder.Flush(); err != nil {
		return "", "XML formatting limit reached; displaying raw source."
	}
	return output.String(), ""
}
