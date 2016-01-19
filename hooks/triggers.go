package hooks

import (
	"github.com/eluleci/dock/utils"
	"net/http"
	"github.com/eluleci/dock/messages"
	"encoding/json"
	"github.com/eluleci/dock/adapters"
	"strings"
	"bytes"
	"mime/multipart"
)

var ExecuteTrigger = func(className, when, method string,
parameters map[string][]string, body map[string]interface{}, multipart *multipart.Form,
user interface{}) (responseBody map[string]interface{}, err *utils.Error) {

	// TODO: optimise adapter not to require redundant instance creation
	adapter := &adapters.MongoAdapter{adapters.MongoDB.C("triggers")}

	whereParams := map[string]interface{}{
		"where": map[string]string{
			"$eq": className,
		},
		"when": map[string]string{
			"$eq": when,
		},
		"method": map[string]string{
			"$eq": strings.ToLower(method),
		},
	}

	whereParamsJson, jsonErr := json.Marshal(whereParams)
	if jsonErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Creating user request failed."}
		return
	}
	requestWrapper := messages.RequestWrapper{}
	requestWrapper.Message.Parameters = make(map[string][]string)
	requestWrapper.Message.Parameters["where"] = []string{string(whereParamsJson)}

	results, fetchErr := adapters.HandleGet(adapter, requestWrapper)
	resultsAsMap := results["data"].([]map[string]interface{})
	if fetchErr != nil || len(resultsAsMap) == 0 {
		return
	}
	triggerData := resultsAsMap[0]

	data := map[string]interface{}{
		"user": user,
		"parameters": parameters,
		"body": body,
		"multipart": multipart,
	}

	var status int
	status, responseBody, err = sendRequest(triggerData["url"].(string), data)

	if status >= 400 {
		err = &utils.Error{status,""}
	}
	return
}

var sendRequest = func(url string, body interface{}) (status int, responseBody map[string]interface{}, err *utils.Error) {

	bodyAsBytes, encodeErr := json.Marshal(body)
	if encodeErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Encoding function request data failed."}
		return
	}

	req, createRequestErr := http.NewRequest("POST", url, bytes.NewBuffer(bodyAsBytes))
	if createRequestErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Creating request to hook server failed."}
		return
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, requestErr := client.Do(req)
	if requestErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Sending request to hook server failed."}
		return
	}
	defer resp.Body.Close()

	status = resp.StatusCode
	decoder := json.NewDecoder(resp.Body)
	decodeErr := decoder.Decode(&responseBody)
	if decodeErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Decoding trigger response body from hook server failed."}
	}
	return
}