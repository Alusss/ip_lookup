package main

import (
	"net/http"
)

func isWebClient(r *http.Request) bool {
	return r.Header.Get("X-Client") == "web"
}


