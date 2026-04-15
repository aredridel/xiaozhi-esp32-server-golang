package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func requestVllm(imagePath, question, url, deviceId string) (string, error) {

	if imagePath == "" || question == "" || url == "" || deviceId == "" {
		return "", fmt.Errorf("usage: main -image <image_path> -question <question> -url <api_url> -device <Device-Id>")
	}

	// Open image file
	file, err := os.Open(imagePath)
	if err != nil {
		fmt.Printf("Failed to open image: %v\n", err)
		return "", err
	}
	defer file.Close()

	// Create multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Write image field
	fileWriter, err := writer.CreateFormFile("file", (imagePath))
	if err != nil {
		fmt.Printf("Failed to create image field: %v\n", err)
		return "", err
	}
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		fmt.Printf("Failed to write image content: %v\n", err)
		return "", err
	}

	// Write text field
	_ = writer.WriteField("question", question)

	writer.Close()

	// Create custom request, add Device-Id header
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Device-Id", deviceId)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response: %v\n", err)
		return "", err
	}
	responseText := string(respBody)
	fmt.Println("Response:")
	fmt.Println(responseText)
	return responseText, nil
}
