# Aprox - A Lightweight HTTP/HTTPS Proxy Server

A fast, simple, and versatile HTTP/HTTPS proxy server written in Go.

## Features

- Handles both HTTP and HTTPS traffic
- No configuration required - works out of the box
- Lightweight and fast
- Cross-platform compatibility
- Simple API for programmatic control

## Installation

### Prerequisites

- Go 1.16 or higher

### Building from source

```bash
git clone https://github.com/yourusername/aprox.git
cd aprox
go build
```

## Usage

### Starting the server

Simply run the executable:

```bash
./aprox
```

The proxy server will start listening on port 9708 by default.

## Client Configuration

### Python with aiohttp

```python
import aiohttp
import asyncio

async def main():
    # Configure the proxy
    proxy = "http://localhost:9708"

    # Create a client session with the proxy
    async with aiohttp.ClientSession() as session:
        # Make a request through the proxy
        async with session.get('https://api.example.com/data',
                              proxy=proxy) as response:
            print(response.status)
            print(await response.text())

asyncio.run(main())
```

### Go with net/http

```go
package main

import (
    "fmt"
    "io/ioutil"
    "net/http"
    "net/url"
)

func main() {
    // Configure the proxy URL
    proxyURL, _ := url.Parse("http://localhost:9708")

    // Create a transport with the proxy
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    }

    // Create a client with the transport
    client := &http.Client{
        Transport: transport,
    }

    // Make a request through the proxy
    resp, err := client.Get("https://api.example.com/data")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    defer resp.Body.Close()

    // Read and print the response
    body, _ := ioutil.ReadAll(resp.Body)
    fmt.Println("Status:", resp.Status)
    fmt.Println("Body:", string(body))
}
```

### Browser Configuration

#### Chrome

1. Go to Settings
2. Search for "proxy" and click on "Open your computer's proxy settings"
3. In Windows:
   - Open "Proxy Settings"
   - Enable "Manual proxy setup"
   - Set HTTP proxy to "localhost" and port to "9708"
4. In macOS:
   - Open System Preferences > Network > Advanced > Proxies
   - Check "Web Proxy (HTTP)" and "Secure Web Proxy (HTTPS)"
   - Set the server to "localhost" and port to "9708"
5. In Linux:
   - Settings vary by distribution, but generally in Network Settings
   - Set HTTP/HTTPS proxy to "localhost:9708"

#### Firefox

1. Go to Settings/Options/Preferences
2. Search for "proxy" and click on "Settings..."
3. Select "Manual proxy configuration"
4. Set "HTTP Proxy" to "localhost" and "Port" to "9708"
5. Check "Also use this proxy for HTTPS"
6. Click "OK"

## Advanced Usage

### Using with curl

```bash
curl -x http://localhost:9708 https://example.com
```

### Using with wget

```bash
wget -e use_proxy=yes -e http_proxy=http://localhost:9708 https://example.com
```

## Troubleshooting

- If you encounter SSL/TLS certificate issues, make sure your client is configured to accept the proxy's certificates
- For connection issues, verify that the proxy server is running and that your client is correctly configured
- For permission issues, ensure the aprox process has network access rights

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
