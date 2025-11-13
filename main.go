package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
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

	query := r.URL.Query()
	from := query.Get("from")
	to := query.Get("to")
	amountStr := query.Get("amount")

	if from == "" || to == "" || amountStr == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if from == to {
		http.Error(w, "Source and target currencies must be different", http.StatusBadRequest)
		return
	}

	if from != strings.ToUpper(from) || to != strings.ToUpper(to) {
		http.Error(w, "Currency codes must be uppercase", http.StatusBadRequest)
		return
	}

	re := regexp.MustCompile(`^[A-Za-z]+$`)
	if !re.MatchString(from) || !re.MatchString(to) {
		http.Error(w, "Currency codes must contain only letters", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	if amount <= 0 {
		http.Error(w, "Amount must be greater than zero", http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/pair/%s/%s/%f", apiKey, from, to, amount)
	res, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making API request:", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("External API error: %s", res.Status), http.StatusInternalServerError)
		return
	}

	var apiResponse struct {
		Result float64 `json:"conversion_result"`
	}
	if err = json.NewDecoder(res.Body).Decode(&apiResponse); err != nil {
		http.Error(w, "Error parsing API response", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"result": apiResponse.Result}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
