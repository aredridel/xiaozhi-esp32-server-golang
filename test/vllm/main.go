package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func main() {
	// Command line parameters
	imagePath := flag.String("image", "", "Image file path")
	question := flag.String("question", "", "Question text")
	url := flag.String("url", "", "HTTP API address")
	deviceId := flag.String("device", "", "Device-Id header")
	flag.Parse()

	if *imagePath == "" || *question == "" || *url == "" || *deviceId == "" {
		fmt.Println("Usage: main -image <image_path> -question <question> -url <api_url> -device <Device-Id>")
		os.Exit(1)
	}

	// Open image file
	file, err := os.Open(*imagePath)
	if err != nil {
		fmt.Printf("Failed to open image: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Write image field
	fileWriter, err := writer.CreateFormFile("file", (*imagePath))
	if err != nil {
		fmt.Printf("Failed to create image field: %v\n", err)
		os.Exit(1)
	}
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		fmt.Printf("Failed to write image content: %v\n", err)
		os.Exit(1)
	}

	// Write text field
	_ = writer.WriteField("question", *question)

	writer.Close()

	// Create custom request, add Device-Id header
	req, err := http.NewRequest("POST", *url, body)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Device-Id", *deviceId)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Response:")
	fmt.Println(string(respBody))
}
