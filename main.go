// main.go
package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	// Start the proxy server
	proxy := &ProxyServer{}
	log.Println("Starting proxy server on :9708")
	log.Fatal(http.ListenAndServe(":9708", proxy))
}

type ProxyServer struct{}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle HTTPS (CONNECT) requests
	if r.Method == http.MethodConnect {
		handleHTTPS(w, r)
	} else {
		// Handle HTTP requests
		handleHTTP(w, r)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract the target host from the original request
	// In a standard proxy request, the Host header contains the final destination
	targetHost := r.Host

	// Create the target URL
	targetURL := &url.URL{
		Scheme: "http", // Default to HTTP
		Host:   targetHost,
		Path:   r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// Create a new director function that correctly sets up the request
	director := func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = targetURL.Path
		req.URL.RawQuery = targetURL.RawQuery

		// Copy all headers from the original request except those we want to modify
		for key, values := range r.Header {
			for _, value := range values {
				// Skip X-Forwarded-For to maintain privacy
				if key != "X-Forwarded-For" {
					req.Header.Add(key, value)
				}
			}
		}

		// Set the Host header to match the target
		req.Host = targetHost

		// Remove any headers that might reveal client information
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Real-IP")
		req.Header.Del("From")
		req.Header.Del("Referer")
	}

	// Create and configure the proxy
	proxy := &httputil.ReverseProxy{
		Director: director,
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	// Establish a TCP tunnel for HTTPS
	destConn, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, "Failed to connect to target", http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	// Hijack the client connection to get a raw TCP connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Send HTTP 200 OK to client to establish the tunnel
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Relay data between client and target
	go func() {
		defer destConn.Close()
		defer clientConn.Close()
		io.Copy(destConn, clientConn)
	}()
	io.Copy(clientConn, destConn)
}