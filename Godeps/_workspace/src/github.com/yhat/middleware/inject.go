package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Inject appends JavaScript to the body of all HTML responses. It buffers all
// responses in memory (so use it selectively). It prints all parsing errors
// using the log package's Print function rather than returning 500s.
func Inject(script string, handler http.Handler) http.Handler {
	hfunc := func(w http.ResponseWriter, r *http.Request) {
		// TODO: accommodate encodings
		r.Header.Del("Accept-Encoding")

		iw := &injectWrapper{
			buf:        bytes.NewBuffer([]byte{}),
			status:     0,
			wr:         w,
			firstWrite: true,
			hijacked:   false,
		}

		// check for hijacker
		var wr http.ResponseWriter
		if hijacker, ok := w.(http.Hijacker); ok {
			wr = &injectWrapperHijacker{iw, hijacker}
		} else {
			wr = iw
		}

		// server request
		handler.ServeHTTP(wr, r)
		if iw.hijacked {
			return
		}

		// default to just writing the request
		b := iw.buf.Bytes()
		writeBody := func() { iw.wr.Write(b) }

		err := func() error {
			// only attempt to parse plain HTML
			get := func(key string) string { return iw.Header().Get(key) }
			if get("Content-Type") != "text/html" || get("Content-Encoding") != "" {
				return nil
			}

			// make a copy of the body
			bcp := make([]byte, len(b))
			copy(bcp, b)

			// attempt to inject the script
			root, err := injectScript(bytes.NewReader(bcp), script)
			if err != nil {
				return fmt.Errorf("middleware: error injecting script: %v", err)
			}

			// render the new html
			buf := bytes.NewBuffer([]byte{})
			if err = html.Render(buf, root); err != nil {
				return fmt.Errorf("middleware: could not render injected html: %v", err)
			}
			iw.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))

			// change the writeBody function to write the altered HTML
			writeBody = func() { io.Copy(iw.wr, buf) }
			return nil
		}()
		// print errors, don't return 500
		if err != nil {
			log.Println(err)
		}

		iw.WriteHeader(iw.status)
		writeBody()
	}
	return http.HandlerFunc(hfunc)
}

func injectScript(r io.Reader, script string) (*html.Node, error) {
	root, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	body, ok := findEle(root, atom.Body)
	if !ok {
		return nil, fmt.Errorf("body not found")
	}

	scriptNode := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Script,
		Data:     "script",
	}
	scriptNode.AppendChild(&html.Node{Type: html.TextNode, Data: script})
	body.AppendChild(scriptNode)
	return root, nil
}

func findEle(n *html.Node, a atom.Atom) (*html.Node, bool) {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n, true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if node, ok := findEle(c, a); ok {
			return node, true
		}
	}
	return nil, false
}

type injectWrapper struct {
	buf        *bytes.Buffer
	status     int
	wr         http.ResponseWriter
	firstWrite bool
	hijacked   bool
}

func (iw *injectWrapper) Header() http.Header {
	return iw.wr.Header()
}

func (iw *injectWrapper) Write(p []byte) (int, error) {
	if iw.status == 0 {
		iw.status = 200
	}
	if iw.firstWrite {
		// TODO: if this is not text/html, no need to buffer
		if "" == iw.Header().Get("Content-Type") {
			contentType := ""
			if len(p) > 512 {
				contentType = http.DetectContentType(p[:512])
			} else {
				contentType = http.DetectContentType(p)
			}
			iw.Header().Set("Content-Type", contentType)
		}
	}
	return iw.buf.Write(p)
}

func (iw *injectWrapper) WriteHeader(status int) {
	iw.status = status
}

type injectWrapperHijacker struct {
	*injectWrapper
	hijacker http.Hijacker
}

func (iw injectWrapperHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	iw.hijacked = true
	return iw.hijacker.Hijack()
}
