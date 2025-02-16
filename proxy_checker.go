package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"gopkg.in/ini.v1"
)

// Config struct to store settings from proxy.config.ini
type Config struct {
	ProxyList string
	Protocol  string
	URL       string
	ValidStr  string
	Timeout   time.Duration
	Threads   int
}

// Load configuration from proxy.config.ini
func loadConfig() (*Config, error) {
	cfg, err := ini.Load("proxy.config.ini")
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy.config.ini: %v", err)
	}

	config := &Config{
		ProxyList: cfg.Section("config").Key("proxy_list").MustString("proxies.txt"),
		Protocol:  cfg.Section("config").Key("protocol").MustString("socks5"),
		URL:       cfg.Section("config").Key("url").MustString("https://example.com"),
		ValidStr:  cfg.Section("config").Key("valid_string").MustString("Success"),
		Timeout:   time.Duration(cfg.Section("config").Key("timeout").MustInt(5000)) * time.Millisecond,
		Threads:   cfg.Section("config").Key("threads").MustInt(10),
	}

	return config, nil
}

// Prompt user for input with a default value
func promptInput(prompt, defaultValue string) string {
	fmt.Printf("%s (Press Enter for default: %s): ", prompt, defaultValue)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	if input == "" {
		return defaultValue
	}
	return input
}

// Prompt user to choose a protocol (Silently includes 4 & 5 as Socks4 and Socks5)
func promptProtocol(defaultProtocol string) string {
	fmt.Println("\nChoose Proxy Protocol:")
	fmt.Println("1. HTTPS")
	fmt.Println("2. SOCKS4")
	fmt.Println("3. SOCKS5")

	protocols := map[string]string{
		"1": "https",
		"2": "socks4",
		"3": "socks5",
		"4": "socks4",
		"5": "socks5",
	}

	choice := promptInput("Enter choice (1-3)", defaultProtocol)
	if val, exists := protocols[choice]; exists {
		return val
	}

	fmt.Println("Invalid choice, using default:", defaultProtocol)
	return defaultProtocol
}

// Fetch proxies from a URL if a URL is given
func fetchProxiesFromURL(proxyURL string) ([]string, error) {
	resp, err := http.Get(proxyURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return extractProxies(string(body)), nil
}

// Extract proxies or URLs from a file or string content
func extractProxies(content string) []string {
	lines := strings.Split(content, "\n")
	proxySet := make(map[string]struct{})

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			proxySet[line] = struct{}{}
		}
	}

	uniqueProxies := make([]string, 0, len(proxySet))
	for proxy := range proxySet {
		uniqueProxies = append(uniqueProxies, proxy)
	}

	return uniqueProxies
}

// Read proxies from a file or fetch from URL
func readProxies(filePath string) ([]string, error) {
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return fetchProxiesFromURL(filePath)
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	proxies := extractProxies(string(data))
	return proxies, nil
}

// Validate proxy by sending request through it
func checkProxy(proxy, protocol, targetURL, validString string, timeout time.Duration, results chan<- string, validCount *int, wg *sync.WaitGroup) {
	defer wg.Done()

	proxyURL := fmt.Sprintf("%s://%s", protocol, proxy)
	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		return
	}

	transport := &http.Transport{Proxy: http.ProxyURL(proxyParsed)}
	client := &http.Client{Transport: transport, Timeout: timeout}

	start := time.Now() // Start measuring response time
	resp, err := client.Get(targetURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	responseTime := time.Since(start).Milliseconds() // Calculate response time

	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	responseBody := string(body[:n])

	if strings.Contains(responseBody, validString) {
		results <- fmt.Sprintf("%s|%dms", proxy, responseTime) // Send proxy with response time
		*validCount++
	}
}

// Save valid proxies to file (without response time)
func saveValidProxy(protocol, proxy string) error {
	os.MkdirAll("results", os.ModePerm)
	file, err := os.OpenFile(fmt.Sprintf("results/%s.valid.txt", protocol), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(proxy + "\n")
	return err
}

// Display valid proxy in green and save it
func handleValidProxy(protocol, proxyWithTime string) {
	parts := strings.Split(proxyWithTime, "|")
	proxy := parts[0]
	responseTime := parts[1]

	fmt.Printf("\033[32m[%s] %s\033[0m\n", responseTime, proxy) // Green text with response time
	saveValidProxy(protocol, proxy)
}

// Function to handle graceful exit
func handleExit(results chan string, validCount *int) {
	go func() {
		for proxy := range results {
			handleValidProxy("socks5", proxy)
		}
		fmt.Printf("\nSaved %d valid proxies to file\n", *validCount)
	}()
}

// Main function
func main() {
	// Print bright blue startup message
	fmt.Println("\033[94mProxy Checker | Fast and Efficient | Written in GO\033[0m")
	fmt.Println("\033[94mBuild Date: Feburary 16 2025...\033[0m\n")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	// Get user inputs with defaults
	proxyFile := promptInput("Enter proxy list file path", config.ProxyList)
	protocol := promptProtocol(config.Protocol)
	threadCount := promptInput("Enter number of threads", fmt.Sprintf("%d", config.Threads))

	// Convert thread count to int
	threads := config.Threads
	if t, err := fmt.Sscanf(threadCount, "%d", &threads); err != nil || t != 1 {
		fmt.Println("Invalid thread count, using default:", config.Threads)
		threads = config.Threads
	}

	// Read proxy list
	proxies, err := readProxies(proxyFile)
	if err != nil {
		fmt.Println("Error reading proxy list:", err)
		return
	}

	// Setup worker pool and progress bar
	var wg sync.WaitGroup
	results := make(chan string, len(proxies))
	bar := progressbar.NewOptions(len(proxies), progressbar.OptionSetWidth(10))

	validCount := 0
	handleExit(results, &validCount)

	sem := make(chan struct{}, threads)
	for _, proxy := range proxies {
		wg.Add(1)
		sem <- struct{}{}
		go func(proxy string) {
			defer func() { <-sem }()
			checkProxy(proxy, protocol, config.URL, config.ValidStr, config.Timeout, results, &validCount, &wg)
			bar.Add(1)
		}(proxy)
	}

	wg.Wait()
	close(results)
	bar.Finish()
	fmt.Printf("\nTotal Online proxies found: %d\n", validCount)
}
