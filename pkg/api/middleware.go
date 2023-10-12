package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type requestResponseDumper struct {
	io.Writer
	http.ResponseWriter
}

// dumpRequestResponse is a middleware that logs the request and response if the
// --debug flag is set. It tries to format the requests and responses as indented JSON
func dumpRequestResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if viper.GetBool("debug") {
			// Request header
			logObject("Request headers", r.Header)
			// Request
			reqBody := []byte{}
			if r.Body != nil { // Read
				reqBody, _ = io.ReadAll(r.Body)
			}
			r.Body = io.NopCloser(bytes.NewBuffer(reqBody)) // Reset
			if len(reqBody) > 0 {
				logObject("Request", reqBody)
			}

			// Response
			resBody := new(bytes.Buffer)
			mw := io.MultiWriter(w, resBody)
			writer := &requestResponseDumper{Writer: mw, ResponseWriter: w}
			w = writer

			next.ServeHTTP(w, r)

			logObject("Response", resBody.Bytes())
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func (w *requestResponseDumper) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *requestResponseDumper) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *requestResponseDumper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *requestResponseDumper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func logObject(prefix string, o interface{}) {
	switch v := o.(type) {
	case string:
		m := map[string]interface{}{}
		err := json.Unmarshal([]byte(v), &m)
		if err == nil {
			logAsJSON(prefix, m)
		} else {
			log.Printf("%s: %v\n", prefix, o)
		}
	case []uint8:
		m := map[string]interface{}{}
		err := json.Unmarshal(v, &m)
		if err == nil {
			logAsJSON(prefix, m)
		} else {
			log.Printf("%s: %v\n", prefix, o)
		}
	default:
		logAsJSON(prefix, o)
	}
}

func logAsJSON(prefix string, o interface{}) {
	data, err := json.MarshalIndent(o, "", "  ")
	if err == nil {
		log.Printf("%s: %s\n", prefix, data)
	} else {
		log.Printf("%s: %v\n", prefix, o)
	}
}
