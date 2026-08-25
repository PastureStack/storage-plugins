package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 16 << 20

type ClientOpts struct {
	URL       string
	AccessKey string
	SecretKey string
	Timeout   time.Duration
}

type Client struct {
	baseURL   *url.URL
	accessKey string
	secretKey string
	http      *http.Client

	Volume        *VolumeClient
	StorageDriver *StorageDriverClient
	Host          *HostClient
}

type ListOpts struct {
	Filters map[string]interface{}
}

type Resource struct {
	Id      string            `json:"id,omitempty"`
	Type    string            `json:"type,omitempty"`
	Links   map[string]string `json:"links,omitempty"`
	Actions map[string]string `json:"actions,omitempty"`
}

type Volume struct {
	Resource
	Driver          string                 `json:"driver,omitempty"`
	DriverOpts      map[string]interface{} `json:"driverOpts,omitempty"`
	HostId          string                 `json:"hostId,omitempty"`
	Mounts          []MountEntry           `json:"mounts,omitempty"`
	Name            string                 `json:"name,omitempty"`
	State           string                 `json:"state,omitempty"`
	StorageDriverId string                 `json:"storageDriverId,omitempty"`
}

type MountEntry struct {
	Path string `json:"path,omitempty"`
}

type VolumeCollection struct {
	Data []Volume `json:"data,omitempty"`
}

type StorageDriver struct {
	Resource
	Name string `json:"name,omitempty"`
}

type StorageDriverCollection struct {
	Data []StorageDriver `json:"data,omitempty"`
}

type Host struct {
	Resource
	Hostname string `json:"hostname,omitempty"`
}

type HostCollection struct {
	Data []Host `json:"data,omitempty"`
}

type schema struct {
	ID    string            `json:"id"`
	Links map[string]string `json:"links"`
}

type schemaCollection struct {
	Data []schema `json:"data"`
}

type VolumeClient struct {
	client     *Client
	collection *url.URL
}

type StorageDriverClient struct {
	client     *Client
	collection *url.URL
}

type HostClient struct {
	client     *Client
	collection *url.URL
}

func NewClient(opts *ClientOpts) (*Client, error) {
	if opts == nil {
		return nil, errors.New("control-plane client options are required")
	}
	baseURL, err := parseBaseURL(opts.URL)
	if err != nil {
		return nil, err
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if timeout < 0 {
		return nil, errors.New("control-plane timeout must not be negative")
	}
	client := &Client{
		baseURL:   baseURL,
		accessKey: opts.AccessKey,
		secretKey: opts.SecretKey,
	}
	client.http = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many control-plane redirects")
			}
			if !client.sameOrigin(request.URL) {
				return errors.New("control-plane redirect changed origin")
			}
			return nil
		},
	}

	collections, err := client.discoverCollections(context.Background())
	if err != nil {
		return nil, err
	}
	volumeURL, err := requiredCollection(collections, "volume")
	if err != nil {
		return nil, err
	}
	storageDriverURL, err := requiredCollection(collections, "storageDriver")
	if err != nil {
		return nil, err
	}
	hostURL, err := requiredCollection(collections, "host")
	if err != nil {
		return nil, err
	}
	client.Volume = &VolumeClient{client: client, collection: volumeURL}
	client.StorageDriver = &StorageDriverClient{client: client, collection: storageDriverURL}
	client.Host = &HostClient{client: client, collection: hostURL}
	return client, nil
}

func parseBaseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("control-plane URL must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("control-plane URL must not contain credentials, a query, or a fragment")
	}
	switch {
	case parsed.Path == "" || parsed.Path == "/":
		parsed.Path = "/v2-beta"
	case parsed.Path == "/v1" || strings.HasPrefix(parsed.Path, "/v1/"):
		parsed.Path = strings.Replace(parsed.Path, "/v1", "/v2-beta", 1)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func (c *Client) sameOrigin(candidate *url.URL) bool {
	return candidate != nil && strings.EqualFold(candidate.Scheme, c.baseURL.Scheme) && strings.EqualFold(candidate.Host, c.baseURL.Host)
}

func (c *Client) resolveTrustedURL(rawURL string) (*url.URL, error) {
	reference, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.New("control-plane returned an invalid URL")
	}
	target := c.baseURL.ResolveReference(reference)
	if !c.sameOrigin(target) {
		return nil, errors.New("control-plane returned a cross-origin URL")
	}
	return target, nil
}

func (c *Client) discoverCollections(ctx context.Context) (map[string]*url.URL, error) {
	request, err := c.newRequest(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connect to control plane: %w", err)
	}
	body, err := readResponse(response)
	if err != nil {
		return nil, err
	}
	schemaHeader := response.Header.Get("X-API-Schemas")
	if schemaHeader == "" {
		return nil, errors.New("control plane did not advertise an API schema")
	}
	schemaURL, err := c.resolveTrustedURL(schemaHeader)
	if err != nil {
		return nil, err
	}
	if schemaURL.String() != c.baseURL.String() {
		if err := c.getJSON(ctx, schemaURL, &body); err != nil {
			return nil, fmt.Errorf("load control-plane schema: %w", err)
		}
	}
	var schemas schemaCollection
	if err := json.Unmarshal(body, &schemas); err != nil {
		return nil, errors.New("control-plane schema response was not valid JSON")
	}
	collections := make(map[string]*url.URL)
	for _, item := range schemas.Data {
		if item.ID == "" || item.Links["collection"] == "" {
			continue
		}
		collectionURL, err := c.resolveTrustedURL(item.Links["collection"])
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", item.ID, err)
		}
		collections[item.ID] = collectionURL
	}
	return collections, nil
}

func requiredCollection(collections map[string]*url.URL, name string) (*url.URL, error) {
	collection := collections[name]
	if collection == nil {
		return nil, fmt.Errorf("control-plane schema does not expose %s", name)
	}
	return collection, nil
}

func (c *Client) newRequest(ctx context.Context, method string, target *url.URL, body io.Reader) (*http.Request, error) {
	if !c.sameOrigin(target) {
		return nil, errors.New("control-plane request target changed origin")
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(c.accessKey, c.secretKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func readResponse(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, errors.New("could not read control-plane response")
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("control-plane response exceeded the size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("control plane returned HTTP %d", response.StatusCode)
	}
	return body, nil
}

func (c *Client) getJSON(ctx context.Context, target *url.URL, destination interface{}) error {
	request, err := c.newRequest(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	body, err := readResponse(response)
	if err != nil {
		return err
	}
	if raw, ok := destination.(*[]byte); ok {
		*raw = body
		return nil
	}
	if err := json.Unmarshal(body, destination); err != nil {
		return errors.New("control-plane response was not valid JSON")
	}
	return nil
}

func (c *Client) modifyJSON(ctx context.Context, method string, target *url.URL, input, output interface{}) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return errors.New("could not encode control-plane request")
	}
	request, err := c.newRequest(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	body, err := readResponse(response)
	if err != nil {
		return err
	}
	if len(body) == 0 || output == nil {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return errors.New("control-plane response was not valid JSON")
	}
	return nil
}

func listURL(collection *url.URL, opts *ListOpts) *url.URL {
	target := *collection
	query := target.Query()
	if opts != nil {
		for key, value := range opts.Filters {
			switch values := value.(type) {
			case []string:
				for _, item := range values {
					query.Add(key, item)
				}
			default:
				query.Add(key, fmt.Sprint(value))
			}
		}
	}
	target.RawQuery = query.Encode()
	return &target
}

func (c *VolumeClient) List(opts *ListOpts) (*VolumeCollection, error) {
	result := &VolumeCollection{}
	err := c.client.getJSON(context.Background(), listURL(c.collection, opts), result)
	return result, err
}

func (c *VolumeClient) Update(existing *Volume, updates interface{}) (*Volume, error) {
	if existing == nil || existing.Links["self"] == "" {
		return nil, errors.New("volume does not contain a self link")
	}
	target, err := c.client.resolveTrustedURL(existing.Links["self"])
	if err != nil {
		return nil, err
	}
	result := &Volume{}
	err = c.client.modifyJSON(context.Background(), http.MethodPut, target, updates, result)
	return result, err
}

func (c *VolumeClient) ActionUpdate(existing *Volume) (*Volume, error) {
	if existing == nil || existing.Actions["update"] == "" {
		return nil, errors.New("volume does not expose the update action")
	}
	target, err := c.client.resolveTrustedURL(existing.Actions["update"])
	if err != nil {
		return nil, err
	}
	result := &Volume{}
	err = c.client.modifyJSON(context.Background(), http.MethodPost, target, struct{}{}, result)
	return result, err
}

func (c *StorageDriverClient) List(opts *ListOpts) (*StorageDriverCollection, error) {
	result := &StorageDriverCollection{}
	err := c.client.getJSON(context.Background(), listURL(c.collection, opts), result)
	return result, err
}

func (c *HostClient) List(opts *ListOpts) (*HostCollection, error) {
	result := &HostCollection{}
	err := c.client.getJSON(context.Background(), listURL(c.collection, opts), result)
	return result, err
}
