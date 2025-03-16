package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	// Get port from environment (Render sets PORT) or default to 10000
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	// Start the proxy server
	proxy := &ProxyServer{}
	log.Printf("Starting proxy server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, proxy))
}

type ProxyServer struct{}

// ServeHTTP handles all incoming requests
func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s from %s", r.Method, r.URL.String(), r.RemoteAddr)

	// Serve a status page at the root
	if r.URL.Path == "/" && r.Method == http.MethodGet && r.Host == r.URL.Host {
		handleRoot(w, r)
		return
	}

	// Handle HTTPS requests (CONNECT or absolute HTTPS URLs)
	if r.Method == http.MethodConnect {
		log.Printf("Handling as HTTPS CONNECT request")
		handleHTTPS(w, r)
		return
	} else if r.URL.IsAbs() && strings.HasPrefix(r.URL.String(), "https://") {
		log.Printf("Handling as HTTPS absolute URL request")
		handleHTTPSAbsoluteURL(w, r)
		return
	}

	// Handle HTTP requests
	log.Printf("Handling as HTTP request")
	handleHTTP(w, r)
}

// handleHTTPS processes HTTPS CONNECT requests using connection hijacking
func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling CONNECT request to %s", r.Host)

	// Connect to the target server
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		log.Printf("Failed to connect to target %s: %v", r.Host, err)
		http.Error(w, "Cannot connect to target", http.StatusBadGateway)
		return
	}

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %v", err)
		destConn.Close()
		return
	}

	// Send 200 Connection Established to the client
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		log.Printf("Failed to send 200 OK: %v", err)
		clientConn.Close()
		destConn.Close()
		return
	}

	// Relay data between client and target
	go func() {
		defer clientConn.Close()
		defer destConn.Close()
		io.Copy(destConn, clientConn)
	}()
	io.Copy(clientConn, destConn)
}

// handleHTTPSAbsoluteURL processes HTTPS requests with absolute URLs
func handleHTTPSAbsoluteURL(w http.ResponseWriter, r *http.Request) {
	var targetURL *url.URL
	var err error

	// Parse the absolute URL in request
	targetURL, err = url.Parse(r.RequestURI)
	if err != nil {
		http.Error(w, "Invalid URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Forwarding HTTPS request to: %s", targetURL.String())

	// Create a new request
	req, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Host = targetURL.Host

	// Configure transport for HTTPS
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// Use system default CA pool (secure)
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to reach target: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}

// handleHTTP processes HTTP requests
func handleHTTP(w http.ResponseWriter, r *http.Request) {
	var targetURL *url.URL
	var err error

	// Parse the target URL from RequestURI
	if strings.HasPrefix(r.RequestURI, "http://") {
		targetURL, err = url.Parse(r.RequestURI)
		if err != nil {
			http.Error(w, "Invalid URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Construct URL from Host and Path
		targetURL = &url.URL{
			Scheme:   "http",
			Host:     r.Host,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}

	log.Printf("Forwarding HTTP request to: %s", targetURL.String())

	// Create a new request
	req, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Host = targetURL.Host

	// Configure transport for HTTP
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to reach target: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}

// handleRoot serves a status page
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>AProx - HTTP/HTTPS Proxy</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        h1 { color: #2c3e50; }
        pre { background-color: #f8f9fa; padding: 10px; border-radius: 4px; }
        .status { padding: 10px; border-radius: 4px; background-color: #d4edda; margin: 20px 0; }
    </style>
</head>
<body>
    <h1>AProx - HTTP/HTTPS Proxy</h1>
    <div class="status">
        <strong>Status:</strong> Running
    </div>
    <h2>Usage</h2>
    <p>Configure your HTTP/HTTPS proxy settings to use this server's address</p>
    <h3>Python Example:</h3>
    <pre>
import requests

proxies = {
    'http': 'http://%s',
    'https': 'http://%s'
}

response = requests.get('https://example.com', proxies=proxies)
print(response.text)
    </pre>
</body>
</html>`, r.Host, r.Host)
}