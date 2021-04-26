package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"

	"github.com/gorilla/websocket"
)

var connectionIds chan string = make(chan string)

var upgrader = websocket.Upgrader{
	Error:       func(w http.ResponseWriter, r *http.Request, status int, reason error) {},
	CheckOrigin: func(r *http.Request) bool { return true },
}

func proxy(w http.ResponseWriter, r *http.Request) {
	clientConnection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade to WS:", err)
		w.Write([]byte("ncalayer-proxy"))
		return
	}
	defer clientConnection.Close()

	connectionId := <-connectionIds
	log.Println("new client connection:", connectionId)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ncalayerConnection, _, err := dialer.Dial("wss://127.0.0.1:13578", nil)
	if err != nil {
		log.Println("failed to connect to NCALayer:", err)
		return
	}
	defer ncalayerConnection.Close()

	fromNCALayer := make(chan string)
	fromClient := make(chan string)
	done := make(chan bool, 2)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}

			mt, msg, readErr := clientConnection.ReadMessage()
			if readErr != nil {
				log.Println("read from client:", readErr)
				done <- true
				done <- true
				return
			}

			if mt != websocket.TextMessage {
				continue
			}

			fromClient <- string(msg)
		}
	}()

	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}

			mt, msg, readErr := ncalayerConnection.ReadMessage()
			if readErr != nil {
				log.Println("read from NCALayer:", readErr)
				done <- true
				done <- true
				return
			}

			if mt != websocket.TextMessage {
				continue
			}

			fromNCALayer <- string(msg)
		}
	}()

	for {
		select {
		case msg := <-fromClient:
			log.Printf("\n%v ---> NCALayer\n", connectionId)
			log.Println(msg)

			if *noConfirmations {
				ncalayerConnection.WriteMessage(websocket.TextMessage, []byte(msg))
			} else {
				for {
					fmt.Print("Send it to NCALayer? [Y]es/[N]o: ")
					resp := ""
					fmt.Scanln(&resp)
					if resp == "Y" || resp == "Yes" {
						ncalayerConnection.WriteMessage(websocket.TextMessage, []byte(msg))
						break
					} else if resp == "N" || resp == "No" {
						break
					}
				}
			}
		case msg := <-fromNCALayer:
			log.Printf("\nNCALayer ---> %v\n", connectionId)
			log.Println(msg)
			if *noConfirmations {
				clientConnection.WriteMessage(websocket.TextMessage, []byte(msg))
			} else {
				for {
					fmt.Print("Send it to Client? [Y]es/[N]o: ")
					resp := ""
					fmt.Scanln(&resp)
					if resp == "Y" || resp == "Yes" {
						clientConnection.WriteMessage(websocket.TextMessage, []byte(msg))
						break
					} else if resp == "N" || resp == "No" {
						break
					}
				}
			}
		case <-done:
			return
		}
	}
}

var port = flag.String("port", "2468", "to listen on")
var noConfirmations = flag.Bool("no-confirmations", false, "turn off confirmations for all messages going through proxy in any direction")

func main() {
	flag.Parse()

	fmt.Printf("Listening on https://127.0.0.1:%v, you have to configure port redirection via firewall like that:\n", *port)
	if runtime.GOOS == "darwin" {
		fmt.Println("echo \"")
		fmt.Printf("rdr pass on lo0 inet proto tcp from any to 127.0.0.1 port 13579 -> 127.0.0.1 port %v\n", *port)
		fmt.Println("rdr pass on lo0 inet proto tcp from any to 127.0.0.1 port 13578 -> 127.0.0.1 port 13579")
		fmt.Println("\" | sudo pfctl -ef -")
	} else {
		fmt.Printf("iptables -t nat -A OUTPUT -o lo -p tcp --dport 13579 -j REDIRECT --to-port %v\n", *port)
		fmt.Println("iptables -t nat -A OUTPUT -o lo -p tcp --dport 13578 -j REDIRECT --to-port 13579")
	}
	fmt.Println("")

	fmt.Println("As long as NCALayer listens on WSS (WebSocket over TLS), this proxy has to behave similarly.")
	fmt.Println("The certificate used for TLS is hardcoded in the app, you have to add exception for it.")
	fmt.Println("To do it, open the following URLs in your browser and configure exceptions:")
	fmt.Println("- https://127.0.0.1:13579")
	fmt.Println("- https://localhost:13579")
	fmt.Println("")

	fmt.Println("To verify that browser communicates with NCALayer via proxy, make shure that the browser displays 'ncalayer-proxy' when you open:")
	fmt.Println("- https://127.0.0.1:13579")
	fmt.Println("- https://localhost:13579")
	fmt.Println("")

	if *noConfirmations {
		fmt.Println("Confirmations are OFF.")
	} else {
		fmt.Println("Confirnations are ON.")
	}
	fmt.Println("")

	go func() {
		id := 0

		for {
			connectionIds <- fmt.Sprintf("%v", id)
			id++
		}
	}()

	http.HandleFunc("/", proxy)

	cert := `
-----BEGIN CERTIFICATE-----
MIIB7jCCAZWgAwIBAgIUTeBhU4hscfsh4ej/fTwwIoZH2WcwCgYIKoZIzj0EAwIw
TTELMAkGA1UEBhMCS1oxDDAKBgNVBAgMA0FsbTEXMBUGA1UECgwObmNhbGF5ZXIt
cHJveHkxFzAVBgNVBAMMDm5jYWxheWVyLXByb3h5MB4XDTIxMDQyMzExMDc1NVoX
DTQ5MDkwODExMDc1NVowTTELMAkGA1UEBhMCS1oxDDAKBgNVBAgMA0FsbTEXMBUG
A1UECgwObmNhbGF5ZXItcHJveHkxFzAVBgNVBAMMDm5jYWxheWVyLXByb3h5MFkw
EwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAErrHrTPGw/APZxFljA7KP2py5W3DBCb2d
lD5EaQCau4SL2hgKZVvm9BzmD4pynYxVDj3Xk9x3eCehf9WuesAdPqNTMFEwHQYD
VR0OBBYEFOoRu+AYsV+/gQQ3bWHis2Ith1HGMB8GA1UdIwQYMBaAFOoRu+AYsV+/
gQQ3bWHis2Ith1HGMA8GA1UdEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwIDRwAwRAIg
fp24L8RQhbwEjBVxC+v/JvaGo5LfwGFA7trDTXmIfOICID3TNfMgGeLFYGwde2Gc
1JCFptd6JU6OYsRzem1Ji+0P
-----END CERTIFICATE-----`

	key := `
-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgSJhFbxzWLU6j/RSs
2RmtLIdymKr2tKt7XTR7sb+ROyihRANCAASusetM8bD8A9nEWWMDso/anLlbcMEJ
vZ2UPkRpAJq7hIvaGAplW+b0HOYPinKdjFUOPdeT3Hd4J6F/1a56wB0+
-----END PRIVATE KEY-----`

	keyPair, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		log.Fatal(err)
	}

	s := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%v", *port),
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{keyPair},
		},
	}
	log.Fatal(s.ListenAndServeTLS("", ""))
}
