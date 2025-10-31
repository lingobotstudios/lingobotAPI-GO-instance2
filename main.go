package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"
)

// HTTPClient reutilizável com connection pooling
var client = &fasthttp.Client{
	MaxConnsPerHost:     1000,
	MaxIdleConnDuration: 90 * time.Second,
	ReadTimeout:         30 * time.Second,
	WriteTimeout:        30 * time.Second,
}

// Middleware de CORS
func withCORS(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if string(ctx.Method()) == fasthttp.MethodOptions {
			ctx.SetStatusCode(fasthttp.StatusNoContent)
			return
		}

		next(ctx)
	}
}

// CallCohere otimizado
func CallCohere(text string) (string, error) {
	apiKey := os.Getenv("COHERE_KEY")
	if apiKey == "" {
		return "", errors.New("cohere API key not configured")
	}

	url := "https://api.cohere.ai/v1/chat"

	payload := map[string]interface{}{
		"message":     text,
		"model":       "command-r",
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	jsonData, _ := sonic.Marshal(payload)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.SetContentType("application/json")
	req.SetBody(jsonData)

	if err := client.Do(req, resp); err != nil {
		return "", err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("cohere API returned status %d", resp.StatusCode())
	}

	var result map[string]interface{}
	if err := sonic.Unmarshal(resp.Body(), &result); err != nil {
		return "", err
	}

	return result["text"].(string), nil
}

// CallGroq otimizado
func CallGroq(text string) (string, error) {
	apiKey := os.Getenv("GROQ_KEY")
	if apiKey == "" {
		return "", errors.New("groq API key not configured")
	}

	url := "https://api.groq.com/openai/v1/chat/completions"

	payload := map[string]interface{}{
		"model": "meta-llama/llama-4-scout-17b-16e-instruct",
		"messages": []map[string]string{
			{"role": "user", "content": text},
		},
		"temperature": 0.7,
	}

	jsonData, _ := sonic.Marshal(payload)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.SetContentType("application/json")
	req.SetBody(jsonData)

	if err := client.Do(req, resp); err != nil {
		return "", err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("groq API returned status %d", resp.StatusCode())
	}

	var result map[string]interface{}
	if err := sonic.Unmarshal(resp.Body(), &result); err != nil {
		return "", err
	}

	choices := result["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	return message["content"].(string), nil
}

// CallOpenRouter otimizado com fallback de modelos
func CallOpenRouter(text string) (string, error) {
	apiKey := os.Getenv("OPENROUTER_KEY")
	if apiKey == "" {
		return "", errors.New("openRouter API key not configured")
	}

	url := "https://openrouter.ai/api/v1/chat/completions"

	modelsToTry := []string{
		"qwen/qwen3-235b-a22b-07-25:free",
		"meta-llama/llama-3.1-8b-instruct:free",
		"microsoft/phi-3-mini-128k-instruct:free",
		"google/gemma-2-9b-it:free",
	}

	for _, model := range modelsToTry {
		payload := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": text},
			},
			"max_tokens":  1000,
			"temperature": 0.7,
		}

		jsonData, _ := sonic.Marshal(payload)

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()

		req.SetRequestURI(url)
		req.Header.SetMethod(fasthttp.MethodPost)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.SetContentType("application/json")
		req.Header.Set("HTTP-Referer", "https://lingobot-api.onrender.com")
		req.Header.Set("X-Title", "Go FastHTTP OpenRouter App")
		req.SetBody(jsonData)

		err := client.Do(req, resp)
		statusCode := resp.StatusCode()

		if err != nil {
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(resp)
			continue
		}

		if statusCode == fasthttp.StatusOK {
			var result map[string]interface{}
			if err := sonic.Unmarshal(resp.Body(), &result); err != nil {
				fasthttp.ReleaseRequest(req)
				fasthttp.ReleaseResponse(resp)
				continue
			}

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(resp)

			choices := result["choices"].([]interface{})
			choice := choices[0].(map[string]interface{})
			message := choice["message"].(map[string]interface{})
			return message["content"].(string), nil
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)

		if statusCode == 503 {
			continue
		}
	}

	return "", errors.New("todos os modelos estão indisponíveis no momento")
}

// CallGemini otimizado
func CallGemini(text string) (string, error) {
	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY1")
	if apiKey == "" {
		return "", errors.New("gemini API key not configured")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", apiKey)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": text},
				},
			},
		},
	}

	jsonData, err := sonic.Marshal(payload)
	if err != nil {
		return "", err
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(jsonData)

	if err := client.Do(req, resp); err != nil {
		return "", err
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf("gemini API returned status %d", resp.StatusCode())
	}

	var result map[string]interface{}
	if err := sonic.Unmarshal(resp.Body(), &result); err != nil {
		return "", err
	}

	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", errors.New("no candidates in response")
	}

	candidate := candidates[0].(map[string]interface{})
	content := candidate["content"].(map[string]interface{})
	parts := content["parts"].([]interface{})
	part := parts[0].(map[string]interface{})

	return part["text"].(string), nil
}

// CallMistral otimizado com retry
func CallMistral(text string) (string, error) {
	apiKey := os.Getenv("MISTRAL_KEY")
	if apiKey == "" {
		return "", errors.New("mistral API key not configured")
	}

	url := "https://api.mistral.ai/v1/chat/completions"
	maxRetries := 3

	payload := map[string]interface{}{
		"model": "mistral-tiny",
		"messages": []map[string]string{
			{"role": "user", "content": text},
		},
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	jsonData, _ := sonic.Marshal(payload)

	for attempt := 0; attempt < maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()

		req.SetRequestURI(url)
		req.Header.SetMethod(fasthttp.MethodPost)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.SetContentType("application/json")
		req.SetBody(jsonData)

		err := client.Do(req, resp)
		statusCode := resp.StatusCode()
		body := resp.Body()

		fasthttp.ReleaseRequest(req)

		if err != nil {
			fasthttp.ReleaseResponse(resp)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			return "", err
		}

		if statusCode == 429 && attempt < maxRetries-1 {
			fasthttp.ReleaseResponse(resp)
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
			continue
		}

		if statusCode != fasthttp.StatusOK {
			fasthttp.ReleaseResponse(resp)
			return "", fmt.Errorf("mistral API returned status %d", statusCode)
		}

		var result map[string]interface{}
		if err := sonic.Unmarshal(body, &result); err != nil {
			fasthttp.ReleaseResponse(resp)
			return "", err
		}

		fasthttp.ReleaseResponse(resp)

		choices := result["choices"].([]interface{})
		choice := choices[0].(map[string]interface{})
		message := choice["message"].(map[string]interface{})
		return message["content"].(string), nil
	}

	return "", errors.New("mistral request failed after retries")
}

// Handler genérico
func createAIHandler(callFunc func(string) (string, error)) func(*fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		if !ctx.IsPost() {
			ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
			ctx.SetBodyString(`{"error":"Method not allowed"}`)
			return
		}

		var req struct {
			Text string `json:"text"`
		}

		if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error":"invalid JSON"}`)
			return
		}

		if req.Text == "" {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error":"text field is required"}`)
			return
		}

		response, err := callFunc(req.Text)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			errMsg, _ := sonic.Marshal(map[string]string{"error": err.Error()})
			ctx.SetBody(errMsg)
			return
		}

		result, _ := sonic.Marshal(map[string]string{"response": response})
		ctx.SetContentType("application/json")
		ctx.SetBody(result)
	}
}

// Handler principal com fallback
func aiHandler(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text         string `json:"text"`
		ForceMistral bool   `json:"force_mistral"`
		ForceCohere  bool   `json:"force_cohere"`
		ForceGroq    bool   `json:"force_groq"`
	}

	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString(`{"error": "invalid JSON"}`)
		return
	}

	var response string
	var err error

	if req.ForceMistral {
		response, err = CallMistral(req.Text)
	} else {
		response, err = CallGemini(req.Text)
		if err != nil {
			response, err = CallMistral(req.Text)
		}
	}

	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		errMsg, _ := sonic.Marshal(map[string]string{"error": err.Error()})
		ctx.SetBody(errMsg)
		return
	}

	result, _ := sonic.Marshal(map[string]string{"response": response})
	ctx.SetContentType("application/json")
	ctx.SetBody(result)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		switch path {
		case "/ai":
			aiHandler(ctx)
		case "/gemini":
			createAIHandler(CallGemini)(ctx)
		case "/mistral":
			createAIHandler(CallMistral)(ctx)
		case "/cohere":
			createAIHandler(CallCohere)(ctx)
		case "/groq":
			createAIHandler(CallGroq)(ctx)
		case "/openrouter":
			createAIHandler(CallOpenRouter)(ctx)
		case "/health":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("OK")
		default:
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			ctx.SetBodyString(`{"error":"endpoint not found"}`)
		}
	}

	addr := ":" + port
	log.Printf("🚀 Server starting on http://localhost%s", addr)
	log.Printf("📍 Endpoints:")
	log.Printf("   - POST /ai          (fallback automático)")
	log.Printf("   - POST /gemini      (Google Gemini)")
	log.Printf("   - POST /mistral     (Mistral AI)")
	log.Printf("   - POST /cohere      (Cohere)")
	log.Printf("   - POST /groq        (Groq)")
	log.Printf("   - POST /openrouter  (OpenRouter)")
	log.Printf("   - GET  /health      (Health check)")
	log.Println()

	if err := fasthttp.ListenAndServe(addr, withCORS(handler)); err != nil {
		log.Fatalf("❌ Error starting server: %v", err)
	}
}
