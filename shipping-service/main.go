package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

const (
	ShippingStatusPending   = "PENDING"
	ShippingStatusShipped   = "SHIPPED"
	ShippingStatusCancelled = "CANCELLED"
)

type Shipping struct {
	ID      string `json:"id"`
	OrderID string `json:"order_id"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

type StartShippingRequest struct {
	OrderID string `json:"order_id"`
	Address string `json:"address"`
}

type ShippingResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ShippingID string `json:"shipping_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

var (
	shippings = make(map[string]Shipping)
	mu        sync.Mutex
	nextID    = 1
)

func main() {
	http.HandleFunc("/start-shipping", startShippingHandler)
	http.HandleFunc("/cancel-shipping", cancelShippingHandler)
	http.HandleFunc("/shipping-status", shippingStatusHandler)

	fmt.Println("Shipping Service started on :8083")
	log.Fatal(http.ListenAndServe(":8083", nil))
}

func startShippingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartShippingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OrderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}
	if req.Address == "" {
		http.Error(w, "Shipping address is required", http.StatusBadRequest)
		return
	}

	shippingSuccess := simulateShippingProcess()

	mu.Lock()
	shippingID := fmt.Sprintf("SHP-%d", nextID)
	nextID++

	status := ShippingStatusPending
	if !shippingSuccess {
		status = ShippingStatusCancelled
	}

	shipping := Shipping{
		ID:      shippingID,
		OrderID: req.OrderID,
		Address: req.Address,
		Status:  status,
	}
	shippings[shippingID] = shipping
	mu.Unlock()

	resp := ShippingResponse{
		Success:    shippingSuccess,
		ShippingID: shippingID,
		OrderID:    req.OrderID,
		Status:     status,
	}

	if shippingSuccess {
		resp.Message = "Shipping initiated successfully"
		w.WriteHeader(http.StatusOK)
	} else {
		resp.Message = "Failed to initiate shipping"
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Shipping initiated: %s for order %s with status %s\n", shippingID, req.OrderID, status)
}

func cancelShippingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	mu.Lock()
	var shippingID string
	var shipping Shipping
	var found bool

	for id, s := range shippings {
		if s.OrderID == req.OrderID && s.Status != ShippingStatusCancelled {
			shippingID = id
			shipping = s
			found = true
			break
		}
	}

	if !found {
		mu.Unlock()
		http.Error(w, "No active shipping found for the order", http.StatusNotFound)
		return
	}

	shipping.Status = ShippingStatusCancelled
	shippings[shippingID] = shipping
	mu.Unlock()

	resp := ShippingResponse{
		Success:    true,
		Message:    "Shipping cancelled successfully",
		ShippingID: shippingID,
		OrderID:    req.OrderID,
		Status:     ShippingStatusCancelled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Shipping cancelled: %s for order %s\n", shippingID, req.OrderID)
}

func shippingStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	var shipping Shipping
	var found bool

	for _, s := range shippings {
		if s.OrderID == orderID {
			shipping = s
			found = true
			break
		}
	}
	mu.Unlock()

	if !found {
		http.Error(w, "No shipping found for the order", http.StatusNotFound)
		return
	}

	resp := ShippingResponse{
		Success:    true,
		ShippingID: shipping.ID,
		OrderID:    orderID,
		Status:     shipping.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func simulateShippingProcess() bool {
	return true
}

func completeShipping(shippingID string) bool {
	mu.Lock()
	defer mu.Unlock()

	shipping, exists := shippings[shippingID]
	if !exists || shipping.Status != ShippingStatusPending {
		return false
	}

	shipping.Status = ShippingStatusShipped
	shippings[shippingID] = shipping
	fmt.Printf("Shipping completed: %s\n", shippingID)
	return true
}
