// main.go
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "9708"
	}

	// Start the proxy server
	proxy := &ProxyServer{}
	log.Println("Starting proxy server on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, proxy))
}

type ProxyServer struct{}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle the root path with a status page
	if r.URL.Path == "/" && r.Method == http.MethodGet && r.Host == r.URL.Host {
		handleRoot(w, r)
		return
	}

	// Handle HTTPS (CONNECT) requests
	if r.Method == http.MethodConnect {
		handleHTTPS(w, r)
	} else {
		// Handle HTTP requests
		handleHTTP(w, r)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	// For a proxy request, the full URL is in the RequestURI
	targetURL, err := url.Parse(r.RequestURI)

	// If this is not a full URL (no scheme), it's a direct request to the proxy
	// rather than a proxy request
	if err != nil || !strings.HasPrefix(r.RequestURI, "http") {
		targetURL = &url.URL{
			Scheme:   "http",
			Host:     r.Host,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}

	// Log the incoming request
	log.Printf("Proxy request: %s %s -> %s", r.Method, r.RequestURI, targetURL.String())

	// Create a new director function that correctly sets up the request
	director := func(req *http.Request) {
		req.URL = targetURL
		// If the Host header is set in the original request, use it
		if r.Host != "" {
			req.Host = r.Host
		} else {
			req.Host = targetURL.Host
		}

		// Copy all headers from the original request except those we want to modify
		for key, values := range r.Header {
			for _, value := range values {
				// Skip certain headers
				if key != "X-Forwarded-For" && key != "Connection" && key != "Proxy-Connection" {
					req.Header.Add(key, value)
				}
			}
		}

		// Remove any headers that might reveal client information
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Real-IP")
		req.Header.Del("From")
		req.Header.Del("Referer")

		// Important: set empty auth
		req.Header.Del("Proxy-Authorization")
	}

	// Create and configure the proxy
	proxy := &httputil.ReverseProxy{
		Director: director,
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	// Log HTTPS connection attempt
	log.Printf("HTTPS CONNECT request to: %s", r.Host)

	// Establish a TCP tunnel for HTTPS
	destConn, err := net.Dial("tcp", r.Host)
	if err != nil {
		log.Printf("Error connecting to target %s: %v", r.Host, err)
		http.Error(w, "Failed to connect to target: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	// Hijack the client connection to get a raw TCP connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %v", err)
		http.Error(w, "Failed to hijack connection: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Send HTTP 200 OK to client to establish the tunnel
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		log.Printf("Failed to send 200 OK to client: %v", err)
		return
	}

	log.Printf("HTTPS tunnel established to %s", r.Host)

	// Relay data between client and target
	go func() {
		defer destConn.Close()
		defer clientConn.Close()
		io.Copy(destConn, clientConn)
	}()
	io.Copy(clientConn, destConn)
}

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