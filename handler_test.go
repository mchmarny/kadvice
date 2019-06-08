package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testHandler(t *testing.T, f *os.File) {
	r := setupRouter()
	req, _ := http.NewRequest("POST", "/test-project/test-cluster", bufio.NewReader(f))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPodCreate(t *testing.T) {
	file, err := os.Open("samples/pod-create.json")
	if err != nil {
		t.Fatal("Pod sample file not found")
	}
	testHandler(t, file)
}

func TestPodDelete(t *testing.T) {
	file, err := os.Open("samples/pod-delete.json")
	if err != nil {
		t.Fatal("Pod sample file not found")
	}
	testHandler(t, file)
}
