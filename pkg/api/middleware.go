package api

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

type logRecord struct {
	http.ResponseWriter
	status int
}

func (r *logRecord) Write(p []byte) (int, error) {
	return r.ResponseWriter.Write(p)
}

func (r *logRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writerWrap := &logRecord{
			ResponseWriter: w,
		}
		next.ServeHTTP(writerWrap, r)
		log.Println(writerWrap.status, r.Method, r.RequestURI)
	})
}
