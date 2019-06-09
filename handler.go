package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	// create a request body copy
	data, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		logger.Printf("Error getting request body: %v", err)
		return
	}

	// print this all out for debugging new event types
	logger.Println(string(data))

	// REST params
	project := c.Param("project")
	cluster := c.Param("cluster")

	// bind payload
	var hook Webhook
	if err := json.Unmarshal(data, &hook); err != nil {
		logger.Printf("Error decoding webhook: %v", err)
		return
	}

	// check that it is not a dry run
	if hook.Request.IsDryRun {
		logger.Println("Webhook result of dry run")
		return
	}

	// filter namespaces
	logger.Printf("NS: %s", hook.Request.Namespace)
	if shouldFilterNS(hook.Request.Namespace) {
		logger.Printf("Skipping, NS filtered out: %s", hook.Request.Namespace)
		return
	}

	// create cluster event
	hr := hook.Request
	ce := &ClusterEvent{
		EventID:   hr.ID,
		Project:   project,
		Cluster:   cluster,
		Namespace: hr.Namespace,
		Operation: hr.Operation,
		EventTime: time.Now().UTC(),
		Content:   data,
	}

	// load object details
	if hr.Object.Kind != "" {
		ce.ObjectKind = hr.Object.Kind
		if hr.Object.Meta.Name != "" {
			ce.Name = hr.Object.Meta.Name
			ce.ObjectID = hr.Object.Meta.ID
			ce.ObjectCreationTime = hr.Object.Meta.CreationOn
			if hr.Object.Meta.Labels.Service != "" {
				ce.Service = hr.Object.Meta.Labels.Service
				ce.Configuration = hr.Object.Meta.Labels.Configuration
				ce.Revision = hr.Object.Meta.Labels.Revision
			}
		}
	} else {
		// capture the deletes name
		ce.Name = hr.Name
		ce.ObjectKind = hr.Kind.Kind
	}

	// printout event for debugging
	logger.Printf("%+v", ce)

	// serialize the whole thing
	ceBytes, err := json.Marshal(ce)
	if err != nil {
		logger.Printf("Error serializing parsed content: %v", err)
		return
	}

	que.push(c.Request.Context(), ceBytes)

	return
}

func shouldFilterNS(ns string) bool {
	for _, n := range excludeNS {
		if ns == n {
			return true
		}
	}
	return false
}

// ClusterEvent represents the extracted event content
type ClusterEvent struct {
	EventID            string    `json:"event_id"`
	Project            string    `json:"project"`
	Cluster            string    `json:"cluster"`
	EventTime          time.Time `json:"event_time"`
	Namespace          string    `json:"namespace"`
	Name               string    `json:"name"`
	Service            string    `json:"service"`
	Revision           string    `json:"revision"`
	Configuration      string    `json:"configuration"`
	Operation          string    `json:"operation"`
	ObjectID           string    `json:"object_id"`
	ObjectKind         string    `json:"object_kind"`
	ObjectCreationTime time.Time `json:"object_creation_time"`
	Content            []byte    `json:"content"`
}

// Webhook represents posted data
type Webhook struct {
	Request struct {
		ID        string `json:"uid"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Operation string `json:"operation"`
		Kind      struct {
			Kind string `json:"kind"`
		} `json:"kind"`
		Object struct {
			Kind string `json:"kind"`
			Meta struct {
				Name       string    `json:"name"`
				ID         string    `json:"uid"`
				CreationOn time.Time `json:"creationTimestamp"`
				Labels     struct {
					Service       string `json:"serving.knative.dev/service"`
					Configuration string `json:"serving.knative.dev/configuration"`
					Revision      string `json:"serving.knative.dev/revision"`
				} `json:"labels"`
			} `json:"metadata"`
		} `json:"object"`
		IsDryRun bool `json:"dryRun"`
	} `json:"request"`
}
