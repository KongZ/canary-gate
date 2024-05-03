package handler

import "net/http"

func HandleHealth(resp http.ResponseWriter, _ *http.Request) {
	resp.WriteHeader(200)
}
