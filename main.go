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
	"time"
)

var apiKey string

func init() {
	loadEnv()
	apiKey = os.Getenv("API_KEY_EXCHANGE")
	if err := validateApiKey(apiKey); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

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

func validateApiKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API_KEY_EXCHANGE environment variable not set")
	}
	return nil
}

func checkCurrencyFormat(s string) bool {
	re := regexp.MustCompile(`^[A-Z]+$`)
	return !re.MatchString(s)
}

type CacheItem struct {
	Data      interface{}
	Timestamp int64
}

var cache = make(map[string]CacheItem)
var cacheTime int64 = 300

func getFromCache(key string) (interface{}, bool) {
	item, ok := cache[key]
	if !ok {
		return nil, false
	}

	if time.Now().Unix()-item.Timestamp > cacheTime {
		delete(cache, key)
		return nil, false
	}
	return item.Data, true
}

func setToCache(key string, data interface{}) {
	cache[key] = CacheItem{
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

func convertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	from := strings.ToUpper(query.Get("from"))
	to := strings.ToUpper(query.Get("to"))
	amountStr := query.Get("amount")

	if from == "" || to == "" || amountStr == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if from == to {
		http.Error(w, "Source and target currencies must be different", http.StatusBadRequest)
		return
	}

	if checkCurrencyFormat(from) || checkCurrencyFormat(to) {
		http.Error(w, "Currency codes must contain only alphabetic letters (no number or symbols)", http.StatusBadRequest)
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

	cacheKey := fmt.Sprintf("convert:%s : %s : %f", from, to, amount)
	if cachedData, found := getFromCache(cacheKey); found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"result": cachedData})
		return
	}
	fmt.Println("Convert request: data feched from API")

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

	setToCache(cacheKey, apiResponse.Result)

	response := map[string]interface{}{"result": apiResponse.Result}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ratesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	base := strings.ToUpper(query.Get("base"))

	if base == "" {
		http.Error(w, "Missing required parameter: base", http.StatusBadRequest)
		return
	}

	if checkCurrencyFormat(base) {
		http.Error(w, "Currency codes must contain only alphabetic letters (no number or symbols)", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("rates:%s", base)
	if cachedData, found := getFromCache(cacheKey); found {
		response := map[string]interface{}{"base": base, "rates": cachedData}
		w.Header().Set("Content-Type", "application/json")
		jsonData, _ := json.MarshalIndent(response, "", "  ")
		w.Write(jsonData)
		return
	}
	fmt.Println("Rates request: data feched from API")

	url := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/%s", apiKey, base)
	res, err := http.Get(url)
	if err != nil {
		http.Error(w, "Error making API request", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("External API error: %s", res.Status), http.StatusInternalServerError)
		return
	}
	var apiResponse struct {
		Rates map[string]float64 `json:"conversion_rates"`
	}

	if err = json.NewDecoder(res.Body).Decode(&apiResponse); err != nil {
		http.Error(w, "Error parsing API response", http.StatusInternalServerError)
		return
	}

	setToCache(cacheKey, apiResponse.Rates)

	response := map[string]interface{}{"base": base, "rates": apiResponse.Rates}
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		http.Error(w, "Error formatting JSON response", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

func main() {

	http.HandleFunc("/convert", convertHandler)
	http.HandleFunc("/rates", ratesHandler)

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
