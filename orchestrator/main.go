package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	OrderServiceURL    = "http://localhost:8081"
	PaymentServiceURL  = "http://localhost:8082"
	ShippingServiceURL = "http://localhost:8083"
)

const (
	TransactionStatusPending   = "PENDING"
	TransactionStatusCompleted = "COMPLETED"
	TransactionStatusFailed    = "FAILED"
)

type Transaction struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	CustomerID    string    `json:"customer_id"`
	Amount        float64   `json:"amount"`
	Address       string    `json:"address"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	FailureReason string    `json:"failure_reason,omitempty"`
	Steps         []Step    `json:"steps"`
}

type Step struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type CreateOrderRequest struct {
	CustomerID string  `json:"customer_id"`
	Items      []Item  `json:"items"`
	Amount     float64 `json:"amount"`
	Address    string  `json:"address"`
}

type Item struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type OrderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	OrderID string `json:"order_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

type PaymentResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	PaymentID string `json:"payment_id,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
	Status    string `json:"status,omitempty"`
}

type ShippingResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ShippingID string `json:"shipping_id,omitempty"`
	OrderID    string `json:"order_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

type TransactionResponse struct {
	Success     bool        `json:"success"`
	Message     string      `json:"message"`
	Transaction Transaction `json:"transaction,omitempty"`
}

var (
	transactions = make(map[string]Transaction)
	mu           sync.Mutex
	nextID       = 1
)

func main() {
	http.HandleFunc("/create-order-saga", createOrderSagaHandler)
	http.HandleFunc("/transaction-status", transactionStatusHandler)

	fmt.Println("Saga Orchestrator started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createOrderSagaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CustomerID == "" {
		http.Error(w, "Customer ID is required", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "Amount must be greater than zero", http.StatusBadRequest)
		return
	}
	if req.Address == "" {
		http.Error(w, "Shipping address is required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	transactionID := fmt.Sprintf("TRX-%d", nextID)
	nextID++

	transaction := Transaction{
		ID:         transactionID,
		CustomerID: req.CustomerID,
		Amount:     req.Amount,
		Address:    req.Address,
		Status:     TransactionStatusPending,
		CreatedAt:  time.Now(),
		Steps:      []Step{},
	}
	transactions[transactionID] = transaction
	mu.Unlock()

	go executeSaga(transactionID, req)

	resp := TransactionResponse{
		Success:     true,
		Message:     "Transaction initiated successfully",
		Transaction: transaction,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)

	fmt.Printf("Transaction initiated: %s\n", transactionID)
}

func transactionStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	transactionID := r.URL.Query().Get("transaction_id")
	if transactionID == "" {
		http.Error(w, "Transaction ID is required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	transaction, exists := transactions[transactionID]
	mu.Unlock()
	if !exists {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	resp := TransactionResponse{
		Success:     true,
		Transaction: transaction,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func executeSaga(transactionID string, req CreateOrderRequest) {
	orderID, err := createOrder(transactionID, req)
	if err != nil {
		updateTransactionStatus(transactionID, TransactionStatusFailed, fmt.Sprintf("Failed to create order: %v", err))
		return
	}

	mu.Lock()
	transaction := transactions[transactionID]
	transaction.OrderID = orderID
	transactions[transactionID] = transaction
	mu.Unlock()

	err = processPayment(transactionID, orderID, req.Amount)
	if err != nil {
		cancelOrder(transactionID, orderID)
		updateTransactionStatus(transactionID, TransactionStatusFailed, fmt.Sprintf("Failed to process payment: %v", err))
		return
	}

	err = startShipping(transactionID, orderID, req.Address)
	if err != nil {
		refundPayment(transactionID, orderID)
		cancelOrder(transactionID, orderID)
		updateTransactionStatus(transactionID, TransactionStatusFailed, fmt.Sprintf("Failed to start shipping: %v", err))
		return
	}

	updateTransactionStatus(transactionID, TransactionStatusCompleted, "")
}

func createOrder(transactionID string, req CreateOrderRequest) (string, error) {
	addStep(transactionID, "CREATE_ORDER")

	orderReq := map[string]interface{}{
		"customer_id": req.CustomerID,
		"items":       req.Items,
		"amount":      req.Amount,
	}
	reqBody, err := json.Marshal(orderReq)
	if err != nil {
		updateStepStatus(transactionID, "CREATE_ORDER", false, err.Error())
		return "", err
	}

	resp, err := http.Post(OrderServiceURL+"/create-order", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "CREATE_ORDER", false, err.Error())
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		updateStepStatus(transactionID, "CREATE_ORDER", false, err.Error())
		return "", err
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		updateStepStatus(transactionID, "CREATE_ORDER", false, err.Error())
		return "", err
	}

	if !orderResp.Success {
		updateStepStatus(transactionID, "CREATE_ORDER", false, orderResp.Message)
		return "", fmt.Errorf(orderResp.Message)
	}

	updateStepStatus(transactionID, "CREATE_ORDER", true, "")

	fmt.Printf("Order created: %s\n", orderResp.OrderID)
	return orderResp.OrderID, nil
}

func processPayment(transactionID, orderID string, amount float64) error {
	addStep(transactionID, "PROCESS_PAYMENT")

	paymentReq := map[string]interface{}{
		"order_id": orderID,
		"amount":   amount,
	}
	reqBody, err := json.Marshal(paymentReq)
	if err != nil {
		updateStepStatus(transactionID, "PROCESS_PAYMENT", false, err.Error())
		return err
	}

	resp, err := http.Post(PaymentServiceURL+"/process-payment", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "PROCESS_PAYMENT", false, err.Error())
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		updateStepStatus(transactionID, "PROCESS_PAYMENT", false, err.Error())
		return err
	}

	var paymentResp PaymentResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		updateStepStatus(transactionID, "PROCESS_PAYMENT", false, err.Error())
		return err
	}

	if !paymentResp.Success {
		updateStepStatus(transactionID, "PROCESS_PAYMENT", false, paymentResp.Message)
		return fmt.Errorf(paymentResp.Message)
	}

	updateStepStatus(transactionID, "PROCESS_PAYMENT", true, "")

	fmt.Printf("Payment processed for order: %s\n", orderID)
	return nil
}

func startShipping(transactionID, orderID, address string) error {
	addStep(transactionID, "START_SHIPPING")

	shippingReq := map[string]interface{}{
		"order_id": orderID,
		"address":  address,
	}
	reqBody, err := json.Marshal(shippingReq)
	if err != nil {
		updateStepStatus(transactionID, "START_SHIPPING", false, err.Error())
		return err
	}

	resp, err := http.Post(ShippingServiceURL+"/start-shipping", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "START_SHIPPING", false, err.Error())
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		updateStepStatus(transactionID, "START_SHIPPING", false, err.Error())
		return err
	}

	var shippingResp ShippingResponse
	if err := json.Unmarshal(body, &shippingResp); err != nil {
		updateStepStatus(transactionID, "START_SHIPPING", false, err.Error())
		return err
	}

	if !shippingResp.Success {
		updateStepStatus(transactionID, "START_SHIPPING", false, shippingResp.Message)
		return fmt.Errorf(shippingResp.Message)
	}

	updateStepStatus(transactionID, "START_SHIPPING", true, "")

	fmt.Printf("Shipping initiated for order: %s\n", orderID)
	return nil
}

func cancelOrder(transactionID, orderID string) {
	addStep(transactionID, "CANCEL_ORDER")

	cancelReq := map[string]interface{}{
		"order_id": orderID,
	}
	reqBody, err := json.Marshal(cancelReq)
	if err != nil {
		updateStepStatus(transactionID, "CANCEL_ORDER", false, err.Error())
		return
	}

	resp, err := http.Post(OrderServiceURL+"/cancel-order", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "CANCEL_ORDER", false, err.Error())
		return
	}
	defer resp.Body.Close()

	updateStepStatus(transactionID, "CANCEL_ORDER", true, "")

	fmt.Printf("Order cancelled: %s\n", orderID)
}

func refundPayment(transactionID, orderID string) {
	addStep(transactionID, "REFUND_PAYMENT")

	refundReq := map[string]interface{}{
		"order_id": orderID,
	}
	reqBody, err := json.Marshal(refundReq)
	if err != nil {
		updateStepStatus(transactionID, "REFUND_PAYMENT", false, err.Error())
		return
	}

	resp, err := http.Post(PaymentServiceURL+"/refund-payment", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "REFUND_PAYMENT", false, err.Error())
		return
	}
	defer resp.Body.Close()

	updateStepStatus(transactionID, "REFUND_PAYMENT", true, "")

	fmt.Printf("Payment refunded for order: %s\n", orderID)
}

func cancelShipping(transactionID, orderID string) {
	addStep(transactionID, "CANCEL_SHIPPING")

	cancelReq := map[string]interface{}{
		"order_id": orderID,
	}
	reqBody, err := json.Marshal(cancelReq)
	if err != nil {
		updateStepStatus(transactionID, "CANCEL_SHIPPING", false, err.Error())
		return
	}

	resp, err := http.Post(ShippingServiceURL+"/cancel-shipping", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		updateStepStatus(transactionID, "CANCEL_SHIPPING", false, err.Error())
		return
	}
	defer resp.Body.Close()

	updateStepStatus(transactionID, "CANCEL_SHIPPING", true, "")

	fmt.Printf("Shipping cancelled for order: %s\n", orderID)
}

func addStep(transactionID, stepName string) {
	mu.Lock()
	defer mu.Unlock()

	transaction, exists := transactions[transactionID]
	if !exists {
		return
	}

	step := Step{
		Name:      stepName,
		Status:    TransactionStatusPending,
		StartedAt: time.Now(),
	}
	transaction.Steps = append(transaction.Steps, step)
	transactions[transactionID] = transaction

	fmt.Printf("Step added to transaction %s: %s\n", transactionID, stepName)
}

func updateStepStatus(transactionID, stepName string, success bool, errorMsg string) {
	mu.Lock()
	defer mu.Unlock()

	transaction, exists := transactions[transactionID]
	if !exists {
		return
	}

	for i, step := range transaction.Steps {
		if step.Name == stepName {
			if success {
				transaction.Steps[i].Status = TransactionStatusCompleted
			} else {
				transaction.Steps[i].Status = TransactionStatusFailed
				transaction.Steps[i].Error = errorMsg
			}
			transaction.Steps[i].EndedAt = time.Now()
			break
		}
	}
	transactions[transactionID] = transaction

	fmt.Printf("Step status updated for transaction %s: %s - %v\n", transactionID, stepName, success)
}

func updateTransactionStatus(transactionID, status, failureReason string) {
	mu.Lock()
	defer mu.Unlock()

	transaction, exists := transactions[transactionID]
	if !exists {
		return
	}

	transaction.Status = status
	if status == TransactionStatusCompleted {
		transaction.CompletedAt = time.Now()
	} else if status == TransactionStatusFailed {
		transaction.FailureReason = failureReason
		transaction.CompletedAt = time.Now()
	}
	transactions[transactionID] = transaction

	fmt.Printf("Transaction status updated: %s - %s\n", transactionID, status)
}
