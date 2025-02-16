package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

// Prompt user to choose a protocol
func promptProtocol(defaultProtocol string) string {
	fmt.Println("\nChoose Proxy Protocol:")
	fmt.Println("1. HTTPS")
	fmt.Println("2. SOCKS4")
	fmt.Println("3. SOCKS5")

	protocols := map[string]string{
		"1": "https",
		"2": "socks4",
		"3": "socks5",
	}

	choice := promptInput("Enter choice (1-3)", defaultProtocol)
	if val, exists := protocols[choice]; exists {
		return val
	}

	fmt.Println("Invalid choice, using default:", defaultProtocol)
	return defaultProtocol
}

// Read proxy list from file
func readProxies(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	return proxies, scanner.Err()
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

	resp, err := client.Get(targetURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	responseBody := string(body[:n])

	if strings.Contains(responseBody, validString) {
		results <- proxy
		*validCount++
	}
}

// Save valid proxies to file
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
func handleValidProxy(protocol, proxy string) {
	fmt.Printf("\033[32m[VALID] %s\033[0m\n", proxy) // Green text output
	saveValidProxy(protocol, proxy)
}

// Function to handle graceful exit
func handleExit(results chan string, validCount *int) {
	// Save proxies if the program exits unexpectedly
	go func() {
		for proxy := range results {
			handleValidProxy("socks5", proxy) // Assuming "socks5" for simplicity, can be changed
		}
		fmt.Printf("\nSaved %d valid proxies to file\n", *validCount)
	}()
}

// Main function
func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	// Get user inputs with defaults
	proxyFile := promptInput("\nEnter proxy list file path", config.ProxyList)
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
	bar := progressbar.NewOptions(len(proxies), progressbar.OptionSetWidth(50), progressbar.OptionSetPredictTime(false), progressbar.OptionSetDescription("Checking proxies"))

	validCount := 0

	// Set up signal handling for graceful exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer close(signalChan)

	// Handle exit and save proxies when exiting
	handleExit(results, &validCount)

	// Start proxy testing with progress bar
	fmt.Println("\nChecking proxies, please wait...\n")
	sem := make(chan struct{}, threads)

	for _, proxy := range proxies {
		wg.Add(1)
		sem <- struct{}{}

		go func(proxy string) {
			defer func() { <-sem }()
			checkProxy(proxy, protocol, config.URL, config.ValidStr, config.Timeout, results, &validCount, &wg)
			bar.Add(1)
			// Update progress bar with valid proxies count
			fmt.Printf("\r[Valid Proxies: %d] ", validCount)
		}(proxy)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process valid proxies
	for proxy := range results {
		handleValidProxy(protocol, proxy)
	}

	// Finish progress bar and display the final count
	bar.Finish()
	fmt.Printf("\nTotal valid proxies found: %d\n", validCount)

	// Wait for a signal (Ctrl+C or SIGTERM)
	<-signalChan
	fmt.Println("\nGraceful shutdown initiated, proxies saved.")
}
