package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

const (
	OrderStatusPending   = "PENDING"
	OrderStatusCompleted = "COMPLETED"
	OrderStatusCancelled = "CANCELLED"
)

type Order struct {
	ID         string  `json:"id"`
	CustomerID string  `json:"customer_id"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status"`
	Items      []Item  `json:"items"`
}

type Item struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type CreateOrderRequest struct {
	CustomerID string  `json:"customer_id"`
	Items      []Item  `json:"items"`
	Amount     float64 `json:"amount"`
}

type OrderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	OrderID string `json:"order_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

var (
	orders = make(map[string]Order)
	mu     sync.Mutex
	nextID = 1
)

func main() {
	http.HandleFunc("/create-order", createOrderHandler)
	http.HandleFunc("/cancel-order", cancelOrderHandler)
	http.HandleFunc("/order-status", orderStatusHandler)

	fmt.Println("Order Service started on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	mu.Lock()
	orderID := fmt.Sprintf("ORD-%d", nextID)
	nextID++

	totalAmount := req.Amount
	if totalAmount == 0 {
		for _, item := range req.Items {
			totalAmount += item.Price * float64(item.Quantity)
		}
	}

	order := Order{
		ID:         orderID,
		CustomerID: req.CustomerID,
		Amount:     totalAmount,
		Status:     OrderStatusPending,
		Items:      req.Items,
	}
	orders[orderID] = order
	mu.Unlock()

	resp := OrderResponse{
		Success: true,
		Message: "Order created successfully",
		OrderID: orderID,
		Status:  OrderStatusPending,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Order created: %s with status %s\n", orderID, OrderStatusPending)
}

func cancelOrderHandler(w http.ResponseWriter, r *http.Request) {
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
	order, exists := orders[req.OrderID]
	if !exists {
		mu.Unlock()
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	order.Status = OrderStatusCancelled
	orders[req.OrderID] = order
	mu.Unlock()

	resp := OrderResponse{
		Success: true,
		Message: "Order cancelled successfully",
		OrderID: req.OrderID,
		Status:  OrderStatusCancelled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Order cancelled: %s\n", req.OrderID)
}

func orderStatusHandler(w http.ResponseWriter, r *http.Request) {
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
	order, exists := orders[orderID]
	mu.Unlock()
	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	resp := OrderResponse{
		Success: true,
		OrderID: orderID,
		Status:  order.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func completeOrder(orderID string) bool {
	mu.Lock()
	defer mu.Unlock()

	order, exists := orders[orderID]
	if !exists {
		return false
	}

	order.Status = OrderStatusCompleted
	orders[orderID] = order
	fmt.Printf("Order completed: %s\n", orderID)
	return true
}
