package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

const (
	PaymentStatusSuccess  = "SUCCESS"
	PaymentStatusFailed   = "FAILED"
	PaymentStatusRefunded = "REFUNDED"
)

type Payment struct {
	ID      string  `json:"id"`
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
	Status  string  `json:"status"`
}

type ProcessPaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type PaymentResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	PaymentID string `json:"payment_id,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
	Status    string `json:"status,omitempty"`
}

var (
	payments = make(map[string]Payment)
	mu       sync.Mutex
	nextID   = 1
)

func main() {
	http.HandleFunc("/process-payment", processPaymentHandler)
	http.HandleFunc("/refund-payment", refundPaymentHandler)
	http.HandleFunc("/payment-status", paymentStatusHandler)

	fmt.Println("Payment Service started on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

func processPaymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OrderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "Amount must be greater than zero", http.StatusBadRequest)
		return
	}

	paymentSuccess := simulatePaymentProcessing(req.Amount)

	mu.Lock()
	paymentID := fmt.Sprintf("PAY-%d", nextID)
	nextID++

	status := PaymentStatusSuccess
	if !paymentSuccess {
		status = PaymentStatusFailed
	}

	payment := Payment{
		ID:      paymentID,
		OrderID: req.OrderID,
		Amount:  req.Amount,
		Status:  status,
	}
	payments[paymentID] = payment
	mu.Unlock()

	resp := PaymentResponse{
		Success:   paymentSuccess,
		PaymentID: paymentID,
		OrderID:   req.OrderID,
		Status:    status,
	}

	if paymentSuccess {
		resp.Message = "Payment processed successfully"
		w.WriteHeader(http.StatusOK)
	} else {
		resp.Message = "Payment processing failed"
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Payment processed: %s for order %s with status %s\n", paymentID, req.OrderID, status)
}

func refundPaymentHandler(w http.ResponseWriter, r *http.Request) {
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
	var paymentID string
	var payment Payment
	var found bool

	for id, p := range payments {
		if p.OrderID == req.OrderID && p.Status == PaymentStatusSuccess {
			paymentID = id
			payment = p
			found = true
			break
		}
	}

	if !found {
		mu.Unlock()
		http.Error(w, "No successful payment found for the order", http.StatusNotFound)
		return
	}

	payment.Status = PaymentStatusRefunded
	payments[paymentID] = payment
	mu.Unlock()

	resp := PaymentResponse{
		Success:   true,
		Message:   "Payment refunded successfully",
		PaymentID: paymentID,
		OrderID:   req.OrderID,
		Status:    PaymentStatusRefunded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Payment refunded: %s for order %s\n", paymentID, req.OrderID)
}

func paymentStatusHandler(w http.ResponseWriter, r *http.Request) {
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
	var payment Payment
	var found bool

	for _, p := range payments {
		if p.OrderID == orderID {
			payment = p
			found = true
			break
		}
	}
	mu.Unlock()

	if !found {
		http.Error(w, "No payment found for the order", http.StatusNotFound)
		return
	}

	resp := PaymentResponse{
		Success:   true,
		PaymentID: payment.ID,
		OrderID:   orderID,
		Status:    payment.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func simulatePaymentProcessing(amount float64) bool {
	return true
}
