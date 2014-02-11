//本包是基于Go标准包中的net/http包下的fs.go修改，无特殊需求无需修改
package httpfileserver

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	//"os"
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// modtime is the modification time of the resource to be served, or IsZero().
// return value is whether this request is now complete.
func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() {
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && modtime.Before(t.Add(1*time.Second)) {
		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}

// checkETag implements If-None-Match and If-Range checks.
// The ETag must have been previously set in the ResponseWriter's headers.
//
// The return value is the effective request "Range" header to use and
// whether this request is now considered done.
func checkETag(w http.ResponseWriter, r *http.Request) (rangeReq string, done bool) {
	etag := w.Header().Get("Etag")
	rangeReq = r.Header.Get("Range")

	// Invalidate the range request if the entity doesn't match the one
	// the client was expecting.
	// "If-Range: version" means "ignore the Range: header unless version matches the
	// current file."
	// We only support ETag versions.
	// The caller must have set the ETag on the response already.
	if ir := r.Header.Get("If-Range"); ir != "" && ir != etag {
		// TODO(bradfitz): handle If-Range requests with Last-Modified
		// times instead of ETags? I'd rather not, at least for
		// now. That seems like a bug/compromise in the RFC 2616, and
		// I've never heard of anybody caring about that (yet).
		rangeReq = ""
	}

	if inm := r.Header.Get("If-None-Match"); inm != "" {
		// Must know ETag.
		if etag == "" {
			return rangeReq, false
		}

		// TODO(bradfitz): non-GET/HEAD requests require more work:
		// sending a different status code on matches, and
		// also can't use weak cache validators (those with a "W/
		// prefix).  But most users of ServeContent will be using
		// it on GET or HEAD, so only support those for now.
		if r.Method != "GET" && r.Method != "HEAD" {
			return rangeReq, false
		}

		// TODO(bradfitz): deal with comma-separated or multiple-valued
		// list of If-None-match values.  For now just handle the common
		// case of a single item.
		if inm == etag || inm == "*" {
			h := w.Header()
			delete(h, "Content-Type")
			delete(h, "Content-Length")
			w.WriteHeader(http.StatusNotModified)
			return "", true
		}
	}
	return rangeReq, false
}

// httpRange specifies the byte range to be sent to the client.
type httpRange struct {
	start, length int64
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

func (r httpRange) mimeHeader(contentType string, size int64) textproto.MIMEHeader {
	return textproto.MIMEHeader{
		"Content-Range": {r.contentRange(size)},
		"Content-Type":  {contentType},
	}
}

// parseRange parses a Range header string as per RFC 2616.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i > size || i < 0 {
				return nil, errors.New("invalid range")
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func sumRangesSize(ranges []httpRange) (size int64) {
	for _, ra := range ranges {
		size += ra.length
	}
	return
}

type countingWriter int64

func (w *countingWriter) Write(p []byte) (n int, err error) {
	*w += countingWriter(len(p))
	return len(p), nil
}

// rangesMIMESize returns the nunber of bytes it takes to encode the
// provided ranges as a multipart response.
func rangesMIMESize(ranges []httpRange, contentType string, contentSize int64) (encSize int64) {
	var w countingWriter
	mw := multipart.NewWriter(&w)
	for _, ra := range ranges {
		mw.CreatePart(ra.mimeHeader(contentType, contentSize))
		encSize += ra.length
	}
	mw.Close()
	encSize += int64(w)
	return
}

func ServeContent(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, size int64, content io.ReadWriter) {
	if checkLastModified(w, r, modtime) {
		return
	}
	rangeReq, done := checkETag(w, r)
	if done {
		return
	}

	code := http.StatusOK

	ctype := w.Header().Get("Content-Type")
	if ctype == "" {
		ctype = mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			var buf [1024]byte
			n, _ := io.ReadFull(content, buf[:])
			b := buf[:n]
			ctype = http.DetectContentType(b)
		}
		w.Header().Set("Content-Type", ctype)
	}

	// 处理 Content-Range 头.
	sendSize := size
	var sendContent io.Reader = content
	if size >= 0 {
		ranges, err := parseRange(rangeReq, size)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if sumRangesSize(ranges) >= size {
			ranges = nil
		}
		switch {
		case len(ranges) == 1:
			ra := ranges[0]
			//if _, err := content.Seek(ra.start, os.SEEK_SET); err != nil {
			//	http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			//	return
			//}
			sendSize = ra.length
			code = http.StatusPartialContent
			w.Header().Set("Content-Range", ra.contentRange(size))
		case len(ranges) > 1:
			for _, ra := range ranges {
				if ra.start > size {
					http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
					return
				}
			}
			sendSize = rangesMIMESize(ranges, ctype, size)
			code = http.StatusPartialContent

			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			w.Header().Set("Content-Type", "multipart/byteranges; boundary="+mw.Boundary())
			sendContent = pr
			defer pr.Close() // cause writing goroutine to fail and exit if CopyN doesn't finish.
			go func() {
				buf := bytes.NewBuffer(make([]byte, 0))
				total := make([]byte, 0)
				content.Read(total)
				for _, ra := range ranges {
					part, err := mw.CreatePart(ra.mimeHeader(ctype, size))
					if err != nil {
						pw.CloseWithError(err)
						return
					}
					buf.Write(total[ra.start : ra.start+ra.length])
					// if _, err := content.Seek(ra.start, os.SEEK_SET); err != nil {
					// 	pw.CloseWithError(err)
					// 	return
					// }
					if _, err := io.CopyN(part, buf, ra.length); err != nil {
						pw.CloseWithError(err)
						return
					}
					buf.Reset()
				}
				mw.Close()
				pw.Close()
			}()
		}

		w.Header().Set("Accept-Ranges", "bytes")
		if w.Header().Get("Content-Encoding") == "" {
			w.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		}
	}

	w.WriteHeader(code)

	if r.Method != "HEAD" {
		io.CopyN(w, sendContent, sendSize)
	}
}
