package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/thedevsaddam/gojsonq.v2"
)

var (
	// TODO: externalize this
	excludeNS = []string{
		"istio-system",
	}
)

func simpleHandler(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// webhookHandler handles the HTTP POST from k8s webhook
// "/:project/:cluster"
func webhookHandler(c *gin.Context) {

	// always success response to the webhook
	c.JSON(http.StatusOK, gin.H{"response": gin.H{"allowed": true}})

	// post payload
	body := c.Request.Body
	data, err := ioutil.ReadAll(body)
	if err != nil {
		logger.Printf("Error getting post data: %v", err)
		return
	}

	// json
	json := string(data)
	logger.Println(json)

	// jq
	jq := gojsonq.New().JSONString(json)

	// skip if not a valid namespace
	ns := getStringValue(jq, "request.namespace")
	if shouldExcludeNamespace(ns) {
		logger.Printf("Skipping, NS filtered out: %s", ns)
		return
	}

	// create core event data
	et := time.Now().UTC()
	e := &Event{
		EventTime: et,
		Project:   c.Param("project"),
		Cluster:   c.Param("cluster"),
		// TODO: Uncomment before pushing to PubSub
		// Content:   data,
		Namespace: ns,
		ID:        getStringValue(jq, "request.uid"),
		Operation: getStringValue(jq, "request.operation"),
		IsDryRun:  getBoolValue(jq, "request.dryRun"),
	}

	// load metadata if exists
	meta := jq.Reset().Find("request.object.metadata")
	if meta != nil {
		e.CreationTime = getTimeValue(jq, "creationTimestamp", et)
		e.Pod = getStringValue(jq, "name")
	}

	// parse labels if exist
	labels := jq.Reset().Find("request.object.metadata.labels")
	if labels != nil {
		m := labels.(map[string]interface{})
		e.Service = asString(m["serving.knative.dev/service"])
		e.Revision = asString(m["serving.knative.dev/revision"])
	}

	// printout event for debugging
	logger.Printf("Event: %+v", e)

	return
}

func shouldExcludeNamespace(ns string) bool {
	for _, n := range excludeNS {
		if ns == n {
			return true
		}
	}
	return false
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	return v.(string)
}

func getStringValue(j *gojsonq.JSONQ, q string) string {
	r := j.Reset().Find(q)
	if r == nil {
		logger.Printf("Unable to find: %s", q)
		return ""
	}
	return r.(string)
}

func getBoolValue(j *gojsonq.JSONQ, q string) bool {
	r := j.Reset().Find(q)
	if r == nil {
		logger.Printf("Unable to find: %s", q)
		return false
	}
	return r.(bool)
}

const timeLayout = "2006-01-02T15:04:05Z"

func getTimeValue(j *gojsonq.JSONQ, q string, d time.Time) time.Time {
	t := getStringValue(j, q)
	if t == "" {
		logger.Printf("Unable to find: %s", q)
		return d
	}

	ts, err := time.Parse(timeLayout, t)
	if err != nil {
		logger.Printf("Error parsing time from: %v", t)
		return d
	}

	return ts
}

func writeResp(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(msg)
}

// Event represents the extracted event content
type Event struct {
	ID           string    `json:"event_id"`
	Project      string    `json:"project"`
	Cluster      string    `json:"cluster"`
	Namespace    string    `json:"namespace"`
	Pod          string    `json:"pod"`
	Service      string    `json:"service"`
	Revision     string    `json:"revision"`
	Operation    string    `json:"operation"`
	CreationTime time.Time `json:"creation_time"`
	Content      []byte    `json:"content"`
	EventTime    time.Time `json:"event_time"`
	IsDryRun     bool      `json:"-,"`
}
