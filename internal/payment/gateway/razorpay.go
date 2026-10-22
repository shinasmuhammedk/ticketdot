package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ticketdot/internal/httpx"
)

const (
	defaultRazorpayBaseURL = "https://api.razorpay.com/v1"
	defaultHTTPTimeout     = 10 * time.Second
	maxResponseBytes       = 1048576 // 1 MB
	minRazorpayAmountPaise = 100     // 100 paise = ₹1.00
	maxReceiptLength       = 40
)

type RazorpayGateway struct {
	keyID      string
	keySecret  string
	baseURL    string
	httpClient *http.Client
}

type RazorpayOption func(*RazorpayGateway)

func WithBaseURL(url string) RazorpayOption {
	return func(g *RazorpayGateway) {
		g.baseURL = strings.TrimRight(url, "/")
	}
}

func WithHTTPClient(client *http.Client) RazorpayOption {
	return func(g *RazorpayGateway) {
		g.httpClient = client
	}
}

func NewRazorpayGateway(keyID, keySecret string, opts ...RazorpayOption) *RazorpayGateway {
	g := &RazorpayGateway{
		keyID:     strings.TrimSpace(keyID),
		keySecret: strings.TrimSpace(keySecret),
		baseURL:   defaultRazorpayBaseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func (g *RazorpayGateway) Name() string {
	return "razorpay"
}

type razorpayOrderRequest struct {
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Receipt  string            `json:"receipt,omitempty"`
	Notes    map[string]string `json:"notes,omitempty"`
}

type razorpayOrderResponse struct {
	ID         string `json:"id"`
	Entity     string `json:"entity"`
	Amount     int64  `json:"amount"`
	AmountDue  int64  `json:"amount_due"`
	AmountPaid int64  `json:"amount_paid"`
	Currency   string `json:"currency"`
	Receipt    string `json:"receipt"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"`
}

type razorpayErrorResponse struct {
	Error struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Source      string `json:"source"`
		Step        string `json:"step"`
		Reason      string `json:"reason"`
	} `json:"error"`
}

func (g *RazorpayGateway) CreateOrder(ctx context.Context, in CreateOrderInput) (*GatewayOrder, error) {
	if in.AmountPaise < minRazorpayAmountPaise {
		return nil, httpx.Validation("amount must be at least 100 paise (₹1.00)")
	}

	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "INR"
	}

	receipt := strings.TrimSpace(in.Receipt)
	if len(receipt) > maxReceiptLength {
		receipt = receipt[:maxReceiptLength]
	}

	// Sanitize notes for Razorpay limits (max 15 pairs, key <= 30 chars, value <= 256 chars)
	sanitizedNotes := make(map[string]string)
	noteCount := 0
	for k, v := range in.Notes {
		if noteCount >= 15 {
			break
		}
		cleanKey := strings.TrimSpace(k)
		if len(cleanKey) > 30 {
			cleanKey = cleanKey[:30]
		}
		cleanVal := strings.TrimSpace(v)
		if len(cleanVal) > 256 {
			cleanVal = cleanVal[:256]
		}
		if cleanKey != "" {
			sanitizedNotes[cleanKey] = cleanVal
			noteCount++
		}
	}

	reqBody := razorpayOrderRequest{
		Amount:   in.AmountPaise,
		Currency: currency,
		Receipt:  receipt,
		Notes:    sanitizedNotes,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/orders", g.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create order request: %w", err)
	}

	// Set HTTP Basic Authentication (Key ID : Key Secret)
	req.SetBasicAuth(g.keyID, g.keySecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		// Log error server-side without leaking credentials or request headers
		log.Printf("[Razorpay Gateway Error] network failure during order creation: %v", err)
		return nil, httpx.BadGateway("payment gateway order creation failed")
	}
	defer resp.Body.Close()

	// Enforce maximum response size limit to protect against denial of service
	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	respBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("[Razorpay Gateway Error] failed to read response body: %v", err)
		return nil, httpx.BadGateway("payment gateway order creation failed")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp razorpayErrorResponse
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error.Code != "" {
			// Log status code and Razorpay error code server-side; NEVER log key secret or Authorization header
			log.Printf("[Razorpay Gateway Error] HTTP %d: code=%s, description=%s, source=%s, reason=%s",
				resp.StatusCode, errResp.Error.Code, errResp.Error.Description, errResp.Error.Source, errResp.Error.Reason)
		} else {
			log.Printf("[Razorpay Gateway Error] HTTP %d: unexpected error response body", resp.StatusCode)
		}
		return nil, httpx.BadGateway("payment gateway order creation failed")
	}

	var orderResp razorpayOrderResponse
	if err := json.Unmarshal(respBytes, &orderResp); err != nil {
		log.Printf("[Razorpay Gateway Error] failed to decode response JSON: %v", err)
		return nil, httpx.BadGateway("payment gateway order creation failed")
	}

	return &GatewayOrder{
		ID:          orderResp.ID,
		AmountPaise: orderResp.Amount,
		Currency:    orderResp.Currency,
		Receipt:     orderResp.Receipt,
		Status:      orderResp.Status,
	}, nil
}

var paymentIDRegex = regexp.MustCompile(`^pay_[A-Za-z0-9]{10,40}$`)

type razorpayPaymentResponse struct {
	ID          string `json:"id"`
	Entity      string `json:"entity"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	OrderID     string `json:"order_id"`
	Method      string `json:"method"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}

func (g *RazorpayGateway) FetchPayment(ctx context.Context, paymentID string) (*GatewayPayment, error) {
	paymentID = strings.TrimSpace(paymentID)
	if !paymentIDRegex.MatchString(paymentID) {
		return nil, httpx.Validation("invalid payment id format")
	}

	endpoint := fmt.Sprintf("%s/payments/%s", g.baseURL, paymentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch payment request: %w", err)
	}

	req.SetBasicAuth(g.keyID, g.keySecret)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		log.Printf("[Razorpay Gateway Error] network failure during payment fetch: %v", err)
		return nil, httpx.BadGateway("payment gateway temporarily unavailable")
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, maxResponseBytes)
	respBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("[Razorpay Gateway Error] failed to read response body: %v", err)
		return nil, httpx.BadGateway("payment gateway temporarily unavailable")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, httpx.NotFound("payment not found on gateway")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp razorpayErrorResponse
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error.Code != "" {
			log.Printf("[Razorpay Gateway Error] HTTP %d: code=%s, description=%s, source=%s, reason=%s",
				resp.StatusCode, errResp.Error.Code, errResp.Error.Description, errResp.Error.Source, errResp.Error.Reason)
		} else {
			log.Printf("[Razorpay Gateway Error] HTTP %d: unexpected error response body", resp.StatusCode)
		}
		if resp.StatusCode >= 500 {
			return nil, httpx.BadGateway("payment gateway temporarily unavailable")
		}
		return nil, httpx.BadRequest("payment gateway rejected payment lookup")
	}

	var pResp razorpayPaymentResponse
	if err := json.Unmarshal(respBytes, &pResp); err != nil {
		log.Printf("[Razorpay Gateway Error] failed to decode payment response JSON: %v", err)
		return nil, httpx.BadGateway("payment gateway response parsing failed")
	}

	return &GatewayPayment{
		ID:          pResp.ID,
		OrderID:     pResp.OrderID,
		AmountPaise: pResp.Amount,
		Currency:    pResp.Currency,
		Status:      pResp.Status,
		Method:      pResp.Method,
	}, nil
}
