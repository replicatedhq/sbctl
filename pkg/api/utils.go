package api

import (
	"strings"
)

// parseAcceptHeader parses the a header and returns a map of key value pairs
// An kubectl Accept header for example looks like this:
// application/json;as=Table;g=meta.k8s.io;v=v1beta1, application/json
func parseAcceptHeader(headers []string) map[string]string {
	m := make(map[string]string)
	for _, header := range headers {

		// Split the header by comma
		d := strings.Split(header, ",")
		for _, dd := range d {

			// Split the header by semicolon
			ss := strings.Split(dd, ";")
			for _, v := range ss {
				// Ignore empty keys
				if v == "" {
					continue
				}

				vv := strings.Split(v, "=")
				if len(vv) == 0 {
					continue
				}

				key := strings.TrimSpace(vv[0])

				// If the key is present, skip it
				_, ok := m[key]
				if ok {
					continue
				}

				// Single key such as "application/json"
				if len(vv) == 1 {
					m[key] = ""
					continue
				}

				// Key-value pair such as "g=meta.k8s.io"
				m[key] = strings.TrimSpace(vv[1])
			}
		}
	}

	return m
}
