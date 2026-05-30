package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/john221wick/gpuSchedularSN/internal/agentserver"
)

type AgentClient interface {
	GetTopology() (*agentserver.TopologyResponse, error)
	PostJob(spec agentserver.JobSpec) error
	GetStatus() (*agentserver.StatusResponse, error)
	GetMonitor() (*agentserver.MonitorResponse, error)
	GetLogs(jobID string, offset int64) (*agentserver.LogChunk, error)
	DeleteJob(jobID string) error
	Ping() error
}

type httpAgentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAgentClient(baseURL string) AgentClient {
	return &httpAgentClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func NewUnixAgentClient(socketPath string) AgentClient {
	return &httpAgentClient{
		baseURL: "http://unix",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}
}

func (c *httpAgentClient) GetTopology() (*agentserver.TopologyResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/topology")
	if err != nil {
		return nil, fmt.Errorf("get topology: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result agentserver.TopologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode topology: %w", err)
	}
	return &result, nil
}

func (c *httpAgentClient) PostJob(spec agentserver.JobSpec) error {
	body, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal job spec: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return c.readError(resp)
	}
	return nil
}

func (c *httpAgentClient) GetStatus() (*agentserver.StatusResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/status")
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result agentserver.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode status: %w", err)
	}
	return &result, nil
}

func (c *httpAgentClient) GetMonitor() (*agentserver.MonitorResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/monitor")
	if err != nil {
		return nil, fmt.Errorf("get monitor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result agentserver.MonitorResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode monitor: %w", err)
	}
	return &result, nil
}

func (c *httpAgentClient) GetLogs(jobID string, offset int64) (*agentserver.LogChunk, error) {
	url := fmt.Sprintf("%s/logs/%s?offset=%d", c.baseURL, jobID, offset)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result agentserver.LogChunk
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode logs: %w", err)
	}
	return &result, nil
}

func (c *httpAgentClient) DeleteJob(jobID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

func (c *httpAgentClient) Ping() error {
	resp, err := c.httpClient.Get(c.baseURL + "/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *httpAgentClient) readError(resp *http.Response) error {
	var errResp agentserver.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("agent error (HTTP %d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("agent error: HTTP %d", resp.StatusCode)
}
