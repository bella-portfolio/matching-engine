package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/libbom14/matching-engine/orderbook"
)

var book = orderbook.New(10000)

type PlaceOrderRequest struct {
	Side     string  `json:"side"`
	Type     string  `json:"type"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	o := &orderbook.Order{
		Side:     orderbook.Side(req.Side),
		Type:     orderbook.OrderType(req.Type),
		Price:    req.Price,
		Quantity: req.Quantity,
	}

	trades, err := book.PlaceOrder(o)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	resp := map[string]interface{}{
		"order":  o,
		"trades": trades,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"missing order id"}`, http.StatusBadRequest)
		return
	}

	o := book.GetOrder(id)
	if o == nil {
		http.Error(w, `{"error":"order not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

func handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if !book.CancelOrder(id) {
		http.Error(w, `{"error":"order not found or not open"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"cancelled"}`))
}

func handleGetTrades(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	trades := book.GetTrades(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trades": trades,
		"count":  len(trades),
	})
}

func handleGetBook(w http.ResponseWriter, r *http.Request) {
	bids, asks := book.BookSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bids": bids,
		"asks": asks,
	})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	trades := book.GetTrades(10000)
	totalQty := 0.0
	totalValue := 0.0
	for _, t := range trades {
		totalQty += t.Quantity
		totalValue += t.Price * t.Quantity
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_trades":  len(trades),
		"total_volume":  totalQty,
		"total_value":   totalValue,
		"avg_price":     func() float64 { if totalQty > 0 { return totalValue / totalQty }; return 0 }(),
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Order endpoints
	mux.HandleFunc("POST /orders", handlePlaceOrder)
	mux.HandleFunc("GET /orders/{id}", handleGetOrder)
	mux.HandleFunc("DELETE /orders/{id}", handleCancelOrder)

	// Market data
	mux.HandleFunc("GET /trades", handleGetTrades)
	mux.HandleFunc("GET /book", handleGetBook)
	mux.HandleFunc("GET /stats", handleStats)

	// Health
	mux.HandleFunc("GET /health", handleHealth)

	handler := loggingMiddleware(mux)

	log.Printf("🚀 Matching Engine starting on :%s (1 core / 2 GB target)", port)
	log.Printf("   POST /orders  — place order")
	log.Printf("   GET  /orders/{id} — get order")
	log.Printf("   DELETE /orders/{id} — cancel order")
	log.Printf("   GET  /trades  — recent trades")
	log.Printf("   GET  /book    — order book snapshot")
	log.Printf("   GET  /stats   — trading stats")
	log.Printf("   GET  /health  — health check")

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
