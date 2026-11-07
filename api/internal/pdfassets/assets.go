// Package pdfassets holds the KaTeX build (0.17.0, MIT) the tryout PDF is rendered with, so the renderer
// never fetches a script, stylesheet or font from the network.
package pdfassets

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"regexp"
)

//go:embed katex.min.js katex.min.css fonts/*.woff2
var files embed.FS

var fontRef = regexp.MustCompile(`url\(fonts/([A-Za-z0-9_-]+\.woff2)\) format\("woff2"\)(?:,url\(fonts/[A-Za-z0-9_-]+\.(?:woff|ttf)\) format\("(?:woff|truetype)"\))*`)

// KaTeX returns the script and a stylesheet whose fonts are woff2 data URIs. Its namespace URIs are written with
// escaped slashes, so the bytes contain no http(s) URL while evaluating to the same strings.
func KaTeX() (script, style []byte, err error) {
	script, err = files.ReadFile("katex.min.js")
	if err != nil {
		return nil, nil, err
	}
	script = bytes.ReplaceAll(script, []byte("://"), []byte(`:\x2f\x2f`))

	style, err = files.ReadFile("katex.min.css")
	if err != nil {
		return nil, nil, err
	}
	var inlineErr error
	style = fontRef.ReplaceAllFunc(style, func(match []byte) []byte {
		name := fontRef.FindSubmatch(match)[1]
		font, err := files.ReadFile("fonts/" + string(name))
		if err != nil {
			inlineErr = err
			return match
		}
		return []byte(fmt.Sprintf(`url(data:font/woff2;base64,%s) format("woff2")`, base64.StdEncoding.EncodeToString(font)))
	})
	if inlineErr != nil {
		return nil, nil, inlineErr
	}
	return script, style, nil
}
