package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func simpleHandler(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// webhookHandler handles the HTTP POST from k8s webhook
// "/:project/:cluster"
func webhookHandler(c *gin.Context) {

	// always success response to the webhook
	c.JSON(http.StatusOK, gin.H{"response": gin.H{"allowed": true}})

	// rest params
	project := c.Param("project")
	cluster := c.Param("cluster")
	logger.Printf("Project: %s Cluster: %s", project, cluster)

	// post payload
	body := c.Request.Body
	data, err := ioutil.ReadAll(body)
	if err != nil {
		logger.Printf("Error getting post data: %v", err)
		return
	}
	logger.Println(string(data))

	// data, _ := json.Marshal(d)
	// err = que.push(r.Context(), data)
	// if err != nil {
	// 	logger.Printf("Error posting data: %v", err)
	// 	writeResp(w, http.StatusBadRequest, "Internal Error")
	// 	return
	// }

	return
}

func writeResp(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(msg)
}

type webhookPost struct {
	Request webhookRequest `json:"request"`
}

type webhookRequest struct {
	ID        string        `json:"uid"`
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Operation string        `json:"operation"`
	Object    webhookObject `json:"object"`
	IsDryRun  bool          `json:"dryRun"`
}

type webhookObject struct {
	Kind      string                `json:"kind"`
	Meta      webhookObjectMetadata `json:"metadata"`
	Spec      objectSpec            `json:"spec"`
	Reason    string                `json:"reason"`
	Message   string                `json:"message"`
	EventType string                `json:"type"`
}

type webhookObjectMetadata struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	CreatedOn time.Time `json:"creationTimestamp"`
}

type objectSpec struct {
	CreatedOn  time.Time   `json:"creationTimestamp"`
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	Containers []container `json:"containers"`
}

type container struct {
	Name         string         `json:"name"`
	ImageURI     string         `json:"image"`
	EnvVars      []containerEnv `json:"env"`
	VolumeMounts []volumeMounts `json:"volumeMounts"`
}

type containerEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type volumeMounts struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}
