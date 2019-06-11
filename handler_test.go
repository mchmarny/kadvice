package main

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testHandler(t *testing.T, f *os.File) {

	parseProject()
	configQueue(context.Background(), project, topic)

	r := setupRouter()
	req, _ := http.NewRequest("POST", "/test-project/test-cluster", bufio.NewReader(f))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPodCreate(t *testing.T) {
	file, err := os.Open("sample/event/pod-create.json")
	if err != nil {
		t.Fatal("Sample file not found: pod-create.json")
	}
	testHandler(t, file)
}

func TestPodDelete(t *testing.T) {
	file, err := os.Open("sample/event/pod-delete.json")
	if err != nil {
		t.Fatal("Sample file not found: pod-delete.json")
	}
	testHandler(t, file)
}

func TestIstioPodCreate(t *testing.T) {
	file, err := os.Open("sample/event/pod-istio-create.json")
	if err != nil {
		t.Fatal("Sample file not found: pod-istio-create.json")
	}
	testHandler(t, file)
}
