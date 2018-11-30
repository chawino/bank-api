package main

import (
	"bankproject/bankapi"
	_ "github.com/lib/pq"
)

func main() {
	bankapi.StartServer()
}
