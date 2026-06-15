// Command gitid manages Git identities by coordinating SSH and Git configuration.
package main

import "fmt"

const version = "0.0.0-dev"

func main() {
	run()
}

// run prints the binary identification line and returns.
func run() {
	fmt.Printf("gitid version %s\n", version)
}
