package tests

import (
	"io"
	"net/http"

	"github.com/pkg/errors"
)

func HTTPExec(verb string, url string, headers map[string]string) (string, int, error) {
	req, err := http.NewRequest(verb, url, nil)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to create http request")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to execute http request")
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to read response body")
	}

	return string(data), res.StatusCode, nil
}
