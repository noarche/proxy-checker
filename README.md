![GO_Proxychecker](https://github.com/user-attachments/assets/b1529eee-859a-452f-b7b4-bac659946402)

# About
Checks proxylist for online proxies. Supports https, socks4 &amp; socks5 protocol. Written in GO.

# Configuration

Default values can be updated by editing `proxy.config.ini`. Default values are set so the user can skip the initial prompts by pressing enter with empty response to questions.

Valid proxies are saved in the `./results/*` directory. 




# To Run: 
`go mod init proxy_checker`

`go mod tidy`

`go get gopkg.in/ini.v1`

`go get github.com/schollz/progressbar/v3`

`go run proxy_checker.go`


# To Build:

`go build -o proxy_checker`

`./proxy_checker`    # On Linux/Mac

`proxy_checker.exe`  # On Windows
