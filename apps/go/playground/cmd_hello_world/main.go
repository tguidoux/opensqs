package main

import (
	"flag"
	"fmt"
)

func main() {
	inputFile := flag.String("input", "", "input")
	verbose := flag.Bool("verbose", false, "verbose")
	help := flag.Bool("help", false, "display help")
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	if *verbose {
		fmt.Println("Verbose mode enabled")
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", "hello_world")
		fmt.Fprintf(flag.CommandLine.Output(), "  -input string\n")
		fmt.Fprintf(flag.CommandLine.Output(), "        input\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  -verbose\n")
		fmt.Fprintf(flag.CommandLine.Output(), "        verbose\n")
	}

	fmt.Println("Hello, World!")
	fmt.Println(*inputFile)
}
