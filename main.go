package main

import (
	"fmt"
	"net/http"
)

func convertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fmt.Fprint(w, "Currency API!")
}

func main() {
	http.HandleFunc("/convert", convertHandler)
	http.ListenAndServe(":8080", nil)
}
