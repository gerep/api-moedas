package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func convertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	apiKey := os.Getenv("API_KEY_EXCHANGE")
	if apiKey == "" {
		http.Error(w, "API key not configured", http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("http://api.exchangeratesapi.io/v1/latest?access_key=%s", apiKey)
	res, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making API request:", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Println("API request failed with status:", res.Status)
		return
	}
	fmt.Println("API response status:", res.Status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)

	_, err = io.Copy(w, res.Body)
	if err != nil {
		fmt.Println("Error copying response body:", err)
	}
}

func main() {
	loadEnv()

	apiKey := os.Getenv("API_KEY_EXCHANGE")
	if apiKey == "" {
		fmt.Println("API_KEY_EXCHANGE environment variable not set")
		return
	}
	http.HandleFunc("/convert", convertHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server is running on port %s...\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
