import requests
import time
import colorama
from colorama import Fore, Style
import os
import re
from tqdm import tqdm
from datetime import datetime

# Initialize colorama
colorama.init(autoreset=True)

# Default directory for saving files
save_directory = "./results"
os.makedirs(save_directory, exist_ok=True)  # Create the directory if it doesn't exist

# Function to extract proxies from URL content
def extract_proxies_from_url(url):
    try:
        response = requests.get(url)
        response.raise_for_status()
        proxies = re.findall(r'\b(?:\d{1,3}\.){3}\d{1,3}:\d{2,5}\b', response.text)
        return proxies
    except requests.exceptions.RequestException as e:
        print(f"{Fore.RED}Failed to fetch proxies from URL: {e}")
        return []

# Function to process multiple URLs from a file
def extract_proxies_from_multiple_urls(file_path):
    all_proxies = set()
    with open(file_path, 'r') as file:
        urls = [line.strip() for line in file if line.strip()]
        for url in tqdm(urls, desc=f"Scraping URLs from {file_path}", unit="url"):
            proxies = extract_proxies_from_url(url)
            all_proxies.update(proxies)
    return list(all_proxies)

# Function to save the list of proxies with the system time and protocol in the filename
def save_proxies_to_file(proxies, protocol):
    current_time = datetime.now().strftime("%Y%m%d_%H%M%S")
    file_name = f"{save_directory}/{current_time}_Proxylist_{protocol}.txt"
    with open(file_name, 'w') as file:
        for proxy in proxies:
            file.write(f"{proxy}\n")
    print(f"{Fore.GREEN}Proxies saved to {file_name}")

# Main function
def main():
    protocols = {
        "https": "https_links.txt",
        "socks4": "socks4_links.txt",
        "socks5": "socks5_links.txt"
    }

    for protocol, file_path in protocols.items():
        if not os.path.exists(file_path):
            print(f"{Fore.YELLOW}File {file_path} not found. Skipping...")
            continue

        print(f"{Fore.CYAN}Processing {file_path} for {protocol.upper()} proxies...")
        proxies = extract_proxies_from_multiple_urls(file_path)
        proxies = list(set(proxies))  # Remove duplicates

        if not proxies:
            print(f"{Fore.RED}No proxies found in {file_path}.")
            continue

        save_proxies_to_file(proxies, protocol)

    # Ask user to continue or exit
    user_choice = input(f"{Fore.YELLOW}Do you want to continue scraping (Y/N)? ").strip().lower()
    if user_choice == 'y':
        main()  # Recursively call main to continue
    else:
        print(f"{Fore.GREEN}Exiting...")

if __name__ == "__main__":
    main()
