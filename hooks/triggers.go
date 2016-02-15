package hooks

import (
	"github.com/eluleci/dock/utils"
	"net/http"
	"encoding/json"
	"github.com/eluleci/dock/adapters"
	"strings"
	"bytes"
	"mime/multipart"
)

var ExecuteFunction = func(res string, parameters map[string][]string, body map[string]interface{}, user interface{}) (responseBody map[string]interface{}, err *utils.Error) {

	originalUrl := res[:strings.LastIndex(res, "-") - 1]
	functionName := res[strings.LastIndex(res, "-") + 1:]

	functionData, tErr := getFunctionData(functionName)
	if tErr != nil {
		err = tErr
		return
	}

	data := map[string]interface{}{
		"res": originalUrl,
		"user": user,
		"parameters": parameters,
		"body": body,
	}

	var status int
	status, responseBody, err = sendRequest(functionData["url"].(string), data)

	if status >= 400 {
		err = &utils.Error{status, ""}
	}
	return
}

var getFunctionData = func(name string) (function map[string]interface{}, err *utils.Error) {

	whereParams := map[string]interface{}{
		"name": map[string]string{
			"$eq": name,
		},
	}

	whereParamsJson, jsonErr := json.Marshal(whereParams)
	if jsonErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Creating 'get function info' request failed."}
		return
	}

	parameters := map[string][]string {"where": []string{string(whereParamsJson)}}
	results, fetchErr := adapters.Query("functions", parameters)

	if fetchErr != nil {
		err = fetchErr
		return
	}
	if results["data"] != nil {
		resultsAsMap := results["data"].([]map[string]interface{})

		if len(resultsAsMap) == 0 {
			err = &utils.Error{http.StatusNotFound, "Function with name '" + name + "' not found."}
			return
		}
		function = resultsAsMap[0]
	}
	return
}

var ExecuteTrigger = func(className, when, method string, parameters map[string][]string, body map[string]interface{}, multipart *multipart.Form, user interface{}) (responseBody map[string]interface{}, err *utils.Error) {

	triggerData, tErr := getTriggerData(className, when, method)
	if tErr != nil {
		if tErr.Code == http.StatusNotFound {
			return
		} else {
			err = tErr
		}
		return
	}

	data := map[string]interface{}{
		"user": user,
		"parameters": parameters,
		"body": body,
		"multipart": multipart,
	}

	var status int
	status, responseBody, err = sendRequest(triggerData["url"].(string), data)

	if status >= 400 {
		err = &utils.Error{status, ""}
	}
	return
}

var getTriggerData = func(className, when, method string) (trigger map[string]interface{}, err *utils.Error) {
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
		err = &utils.Error{http.StatusInternalServerError, "Creating 'get trigger info' request failed."}
		return
	}

	parameters := map[string][]string {"where": []string{string(whereParamsJson)}}
	results, fetchErr := adapters.Query("triggers", parameters)

	if fetchErr != nil {
		err = fetchErr
		return
	}
	if results["data"] != nil {
		resultsAsMap := results["data"].([]map[string]interface{})

		if len(resultsAsMap) == 0 {
			err = &utils.Error{http.StatusNotFound, "Trigger not found."}
			return
		}
		trigger = resultsAsMap[0]
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