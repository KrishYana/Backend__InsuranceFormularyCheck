package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"hello": "world"}
	JSON(w, http.StatusOK, data)

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if ct := result.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}

	var envelope Envelope
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	dataMap, ok := envelope.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be map, got %T", envelope.Data)
	}
	if dataMap["hello"] != "world" {
		t.Errorf("expected data.hello = 'world', got '%v'", dataMap["hello"])
	}
	if envelope.Meta != nil {
		t.Errorf("expected Meta nil, got %v", envelope.Meta)
	}
}

func TestJSONWithMeta(t *testing.T) {
	w := httptest.NewRecorder()

	data := []string{"a", "b", "c"}
	meta := Meta{Count: 3, Page: 1}
	JSONWithMeta(w, http.StatusOK, data, meta)

	result := w.Result()
	defer result.Body.Close()

	var envelope Envelope
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if envelope.Meta == nil {
		t.Fatal("expected Meta non-nil")
	}
	if envelope.Meta.Count != 3 {
		t.Errorf("expected Meta.Count 3, got %d", envelope.Meta.Count)
	}
	if envelope.Meta.Page != 1 {
		t.Errorf("expected Meta.Page 1, got %d", envelope.Meta.Page)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid input")

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", result.StatusCode)
	}

	var body ErrorBody
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if body.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected error code 'BAD_REQUEST', got '%s'", body.Error.Code)
	}
	if body.Error.Message != "invalid input" {
		t.Errorf("expected error message 'invalid input', got '%s'", body.Error.Message)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()

	NotFound(w, "drug not found")

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", result.StatusCode)
	}

	var body ErrorBody
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("expected code 'NOT_FOUND', got '%s'", body.Error.Code)
	}
	if body.Error.Message != "drug not found" {
		t.Errorf("expected message 'drug not found', got '%s'", body.Error.Message)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()

	BadRequest(w, "missing rxcui parameter")

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", result.StatusCode)
	}

	var body ErrorBody
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected code 'BAD_REQUEST', got '%s'", body.Error.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()

	Unauthorized(w, "invalid token")

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", result.StatusCode)
	}

	var body ErrorBody
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code 'UNAUTHORIZED', got '%s'", body.Error.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()

	InternalError(w)

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", result.StatusCode)
	}

	var body ErrorBody
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if body.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected code 'INTERNAL_ERROR', got '%s'", body.Error.Code)
	}
	if body.Error.Message != "An unexpected error occurred" {
		t.Errorf("expected generic error message, got '%s'", body.Error.Message)
	}
}

func TestJSON_CustomStatus(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusCreated, map[string]int{"id": 42})

	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", result.StatusCode)
	}
}

func TestJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusOK, nil)

	result := w.Result()
	defer result.Body.Close()

	var envelope Envelope
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if envelope.Data != nil {
		t.Errorf("expected data nil, got %v", envelope.Data)
	}
}

func TestEnvelope_JSONKeys(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, "test")

	var raw map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if _, ok := raw["data"]; !ok {
		t.Error("expected 'data' key in JSON response")
	}
}

func TestErrorBody_JSONKeys(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, http.StatusBadRequest, "TEST", "test message")

	var raw map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	errObj, ok := raw["error"]
	if !ok {
		t.Fatal("expected 'error' key in JSON response")
	}

	errMap, ok := errObj.(map[string]interface{})
	if !ok {
		t.Fatalf("expected error to be object, got %T", errObj)
	}

	if _, ok := errMap["code"]; !ok {
		t.Error("expected 'code' key in error object")
	}
	if _, ok := errMap["message"]; !ok {
		t.Error("expected 'message' key in error object")
	}
}
