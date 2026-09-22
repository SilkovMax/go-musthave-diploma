package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ответ от внешнего сервиса начислений
type OrderResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// HTTP-клиент для взаимодействия с внешним сервисом начислений
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Запрашивает информацию о начислении для конкретного заказа
func (c *Client) GetOrderStatus(ctx context.Context, orderNumber string) (*OrderResponse, error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("выполнение запроса: %w", err)
	}
	defer resp.Body.Close()

	//429
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("%w: Retry-After=%s", ErrTooManyRequests, retryAfter)
	}

	//204
	if resp.StatusCode == http.StatusNoContent {
		return nil, ErrOrderNotFound
	}

	//500
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("статус ответа от сервера: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("чтение ответа: %w", err)
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("декодирование JSON: %w", err)
	}

	return &orderResp, nil
}

// парсит заголовок Retry-After и возвращает длительность паузы
func ParseRetryAfter(retryAfterStr string) time.Duration {
	if retryAfterStr == "" {
		return 60 * time.Second
	}

	seconds, err := strconv.Atoi(retryAfterStr)
	if err != nil || seconds <= 0 {
		return 60 * time.Second
	}

	return time.Duration(seconds) * time.Second
}

var (
	ErrTooManyRequests = errors.New("превышен лимит запросов")
	ErrOrderNotFound   = errors.New("заказ не найден во внешнем сервисе")
)
