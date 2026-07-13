package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

type RHClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewRHClient(baseURL string) *RHClient {
	return &RHClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ForBaseURL returns a new RHClient targeting a different base URL but sharing the HTTP client.
func (c *RHClient) ForBaseURL(baseURL string) *RHClient {
	if baseURL == "" || baseURL == c.BaseURL {
		return c
	}
	return &RHClient{
		BaseURL:    baseURL,
		HTTPClient: c.HTTPClient,
	}
}

// host extracts the hostname from the base URL for the Host header.
func (c *RHClient) host() string {
	if u, err := url.Parse(c.BaseURL); err == nil {
		return u.Host
	}
	return "www.runninghub.cn"
}

// CreateTaskRequest matches RunningHub's task creation body
type CreateTaskRequest struct {
	ApiKey         string          `json:"apiKey"`
	WebappID       string          `json:"webappId"`
	NodeInfoList   json.RawMessage `json:"nodeInfoList"`
	WebhookURL     string          `json:"webhookUrl,omitempty"`
	InstanceType   string          `json:"instanceType,omitempty"`
	AccessPassword string          `json:"accessPassword,omitempty"`
}

// CreateTaskResponse matches RunningHub's response
type CreateTaskResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		NetWssURL  string `json:"netWssUrl"`
		TaskID     string `json:"taskId"`
		ClientID   string `json:"clientId"`
		TaskStatus string `json:"taskStatus"`
		PromptTips string `json:"promptTips"`
	} `json:"data"`
}

// QueryTaskResponse matches RunningHub V2 query response
type QueryTaskResponse struct {
	TaskID       string          `json:"taskId"`
	Status       string          `json:"status"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Results      json.RawMessage `json:"results"`
	ClientID     string          `json:"clientId"`
	PromptTips   string          `json:"promptTips"`
}

// UploadResponse matches RunningHub's upload response
type UploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		FileName string `json:"fileName"`
		FileType string `json:"fileType"`
	} `json:"data"`
}

func (c *RHClient) CreateTask(apiKey string, req CreateTaskRequest) (*CreateTaskResponse, error) {
	req.ApiKey = apiKey

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/task/openapi/ai-app/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Host", c.host())
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result CreateTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &result, nil
}

func (c *RHClient) QueryTask(apiKey string, taskID string) (*QueryTaskResponse, error) {
	body, _ := json.Marshal(map[string]string{"taskId": taskID})

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/openapi/v2/query", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result QueryTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &result, nil
}

func (c *RHClient) UploadFile(apiKey string, fileType string, file multipart.File, filename string) (*UploadResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("apiKey", apiKey)
	_ = writer.WriteField("fileType", fileType)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}
	writer.Close()

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/task/openapi/upload", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Host", c.host())

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result UploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &result, nil
}

func (c *RHClient) GetWebappInfo(apiKey string, webappID string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/api/webapp/apiCallDemo?apiKey=%s&webappId=%s", c.BaseURL, apiKey, webappID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return respBody, nil
}

// CancelTaskResponse matches RunningHub cancel response
type CancelTaskResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (c *RHClient) CancelTask(apiKey string, taskID string) (*CancelTaskResponse, error) {
	body, _ := json.Marshal(map[string]string{"apiKey": apiKey, "taskId": taskID})

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/task/openapi/cancel", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Host", c.host())
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result CancelTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &result, nil
}
