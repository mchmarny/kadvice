package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	meta "cloud.google.com/go/compute/metadata"
	"github.com/gin-gonic/gin"
)

const (
	appName     = "kadvice"
	notSetValue = "none"
	defaultPort = "8080"
)

var (
	logger  = log.New(os.Stdout, "[ka] ", 0)
	project = mustEnvVar("PROJECT", notSetValue)
	topic   = mustEnvVar("TOPIC", appName)
	port    = mustEnvVar("PORT", defaultPort)
	que     *queue
)

func main() {

	// config
	project = ensureProject()
	que = getQueue(context.Background(), project, topic)

	// server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Server starting: %s \n", addr)
	if err := setupRouter().Run(addr); err != nil {
		log.Fatal(err)
	}

}

func setupRouter() *gin.Engine {

	// router
	gin.SetMode(gin.ReleaseMode)

	// root, post, health handlers
	r := gin.New()
	r.GET("/", simpleHandler)
	r.GET("/health", simpleHandler)
	r.POST("/:project/:cluster", webhookHandler)

	return r

}

func ensureProject() string {
	if project != notSetValue {
		return project
	}

	mc := meta.NewClient(&http.Client{Transport: userAgentTransport{
		userAgent: appName,
		base:      http.DefaultTransport,
	}})
	p, err := mc.ProjectID()
	if err != nil {
		logger.Fatalf("Error creating metadata client: %v", err)
	}
	return p
}

func mustEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		logger.Printf("%s: %s", key, val)
		return strings.TrimSpace(val)
	}

	if fallbackValue == "" {
		logger.Fatalf("Required envvar not set: %s", key)
	}

	logger.Printf("%s: %s (not set, using default)", key, fallbackValue)
	return fallbackValue
}

// GCP Metadata
// https://godoc.org/cloud.google.com/go/compute/metadata#example-NewClient
type userAgentTransport struct {
	userAgent string
	base      http.RoundTripper
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(req)
}
