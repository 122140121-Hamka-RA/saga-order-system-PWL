package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	OrchestratorURL = "http://localhost:8080"
)

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

type TransactionResponse struct {
	Success     bool        `json:"success"`
	Message     string      `json:"message"`
	Transaction Transaction `json:"transaction,omitempty"`
}

type Transaction struct {
	ID            string  `json:"id"`
	OrderID       string  `json:"order_id"`
	CustomerID    string  `json:"customer_id"`
	Amount        float64 `json:"amount"`
	Address       string  `json:"address"`
	Status        string  `json:"status"`
	FailureReason string  `json:"failure_reason,omitempty"`
	Steps         []Step  `json:"steps"`
}

type Step struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func main() {
	fmt.Println("=== Running Success Scenario ===")
	runSuccessScenario()

	fmt.Println("\n=== Running Payment Failure Scenario ===")
	runPaymentFailureScenario()

	fmt.Println("\n=== Running Shipping Failure Scenario ===")
	runShippingFailureScenario()
}

func runSuccessScenario() {
	req := CreateOrderRequest{
		CustomerID: "customer-123",
		Items: []Item{
			{
				ID:       "item-1",
				Name:     "Product A",
				Price:    100.0,
				Quantity: 2,
			},
		},
		Amount:  200.0,
		Address: "123 Main St, City, Country",
	}

	transactionID := createOrder(req)
	if transactionID == "" {
		fmt.Println("Failed to create order")
		return
	}

	fmt.Println("Waiting for transaction to complete...")
	checkTransactionStatus(transactionID)
}

func runPaymentFailureScenario() {
	req := CreateOrderRequest{
		CustomerID: "customer-456",
		Items: []Item{
			{
				ID:       "item-2",
				Name:     "Product B",
				Price:    50.0,
				Quantity: 1,
			},
		},
		Amount:  0.0,
		Address: "456 Second St, City, Country",
	}

	transactionID := createOrder(req)
	if transactionID == "" {
		fmt.Println("Failed to create order")
		return
	}

	fmt.Println("Waiting for transaction to complete...")
	checkTransactionStatus(transactionID)
}

func runShippingFailureScenario() {
	req := CreateOrderRequest{
		CustomerID: "customer-789",
		Items: []Item{
			{
				ID:       "item-3",
				Name:     "Product C",
				Price:    150.0,
				Quantity: 1,
			},
		},
		Amount:  150.0,
		Address: "",
	}

	transactionID := createOrder(req)
	if transactionID == "" {
		fmt.Println("Failed to create order")
		return
	}

	fmt.Println("Waiting for transaction to complete...")
	checkTransactionStatus(transactionID)
}

func createOrder(req CreateOrderRequest) string {
	reqBody, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return ""
	}

	resp, err := http.Post(OrchestratorURL+"/create-order-saga", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return ""
	}

	var transactionResp TransactionResponse
	if err := json.Unmarshal(body, &transactionResp); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		return ""
	}

	if !transactionResp.Success {
		fmt.Printf("Transaction creation failed: %s\n", transactionResp.Message)
		return ""
	}

	fmt.Printf("Transaction created: %s\n", transactionResp.Transaction.ID)
	return transactionResp.Transaction.ID
}

func checkTransactionStatus(transactionID string) {
	resp, err := http.Get(fmt.Sprintf("%s/transaction-status?transaction_id=%s", OrchestratorURL, transactionID))
	if err != nil {
		fmt.Printf("Error getting transaction status: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	var transactionResp TransactionResponse
	if err := json.Unmarshal(body, &transactionResp); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		return
	}

	fmt.Printf("Transaction ID: %s\n", transactionResp.Transaction.ID)
	fmt.Printf("Status: %s\n", transactionResp.Transaction.Status)
	if transactionResp.Transaction.FailureReason != "" {
		fmt.Printf("Failure Reason: %s\n", transactionResp.Transaction.FailureReason)
	}

	fmt.Println("Steps:")
	for _, step := range transactionResp.Transaction.Steps {
		fmt.Printf("  - %s: %s\n", step.Name, step.Status)
		if step.Error != "" {
			fmt.Printf("    Error: %s\n", step.Error)
		}
	}
}
