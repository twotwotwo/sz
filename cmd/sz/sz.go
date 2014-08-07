// cmd/sz is a toy program for playing with github.com/twotwotwo/sz.
// Note it'll happily write compressed output to your terminal.
package main

import (
	"fmt"
	"github.com/twotwotwo/sz"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(os.Args[0], "usage:", os.Args[0], "[filename]")
		os.Exit(1)
	}
	in, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println("error opening input:", err)
		os.Exit(1)
	}
	unpack := strings.HasSuffix(os.Args[1], ".sz")
	if unpack {
		r, err := sz.NewReaderStrictMem(in)
		if err != nil {
			fmt.Println("error creating reader:", err)
			os.Exit(1)
		}
		_, err = io.Copy(os.Stdout, r)
		if err != nil {
			fmt.Println("error decompressing:", err)
			os.Exit(1)
		}
	} else {
		w, err := sz.NewWriter(os.Stdout)
		if err != nil {
			fmt.Println("error creating writer:", err)
			os.Exit(1)
		}
		_, err = io.Copy(w, in)
		w.Flush()
		if err != nil {
			fmt.Println("error compressing:", err)
			os.Exit(1)
		}
	}
}
