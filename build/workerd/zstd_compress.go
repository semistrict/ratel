package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/klauspost/compress/zstd"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s input output\n", os.Args[0])
		os.Exit(2)
	}

	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading input: %v\n", err)
		os.Exit(1)
	}

	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating zstd encoder: %v\n", err)
		os.Exit(1)
	}
	if _, err := encoder.Write(input); err != nil {
		encoder.Close()
		fmt.Fprintf(os.Stderr, "compressing input: %v\n", err)
		os.Exit(1)
	}
	if err := encoder.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "finalizing compressed output: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(os.Args[2], compressed.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "writing output: %v\n", err)
		os.Exit(1)
	}
}
