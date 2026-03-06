package main

import (
	"bytes"
	"net/http"
	"os"
)

func sendToRedPill(message string) {
	token := os.Getenv("RED_PILL_TOKEN")
	url := "https://api.red-pill.ai/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(message))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

}

func main() {

}
