package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

func main() {
	// Create a simple test PNG image (blue square)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{0, 100, 200, 255})
		}
	}
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		log.Fatal(err)
	}

	realImgPath := "./uploads/504b98ea-4849-4526-be29-e88e95e8a581.jpg"
	var imageData []byte
	var filename string
	data, err := os.ReadFile(realImgPath)
	if err == nil {
		imageData = data
		filename = "ps5.jpg"
		fmt.Printf("Using real image: %s (%d bytes)\n", realImgPath, len(data))
	} else {
		imageData = imgBuf.Bytes()
		filename = "test_blue.png"
		fmt.Printf("Using generated test image (%d bytes)\n", len(imageData))
	}

	mode := "reward"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// Build multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		log.Fatal(err)
	}
	part.Write(imageData)
	writer.WriteField("language", "he")
	writer.WriteField("mode", mode)
	writer.Close()

	// We need a valid auth token - let's login first
	loginBody := `{"username":"koko","password":"koko"}`
	loginResp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewBufferString(loginBody))
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer loginResp.Body.Close()
	loginData, _ := io.ReadAll(loginResp.Body)

	var session map[string]interface{}
	json.Unmarshal(loginData, &session)

	groupID, _ := session["choregroup_id"].(string)
	if groupID == "" {
		log.Fatalf("No choregroup_id in login response: %s", string(loginData))
	}
	fmt.Printf("Logged in. Group ID: %s\n", groupID)

	// Get token from cookies
	var cookies []*http.Cookie
	cookies = loginResp.Cookies()

	// Send AI analyze request
	url := fmt.Sprintf("http://localhost:8080/api/choregroups/%s/ai/analyze-image", groupID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("\nStatus: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(respBody))
}
