package main

import (
	"flag"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

func main() {
	var clientId, clientSecret string
	flag.StringVar(&clientId, "client_id", "", "your-client-id")
	flag.StringVar(&clientSecret, "client_secret", "", "your-client-secret")

	flag.Parse()

	RunBotListener(clientId, clientSecret)

	select {}
}
