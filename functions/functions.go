package functions
import (
	"github.com/eluleci/dock/config"
	"github.com/eluleci/dock/utils"
	"net/http"
	"github.com/eluleci/dock/messages"
	"bytes"
	"encoding/json"
)

var ExecuteCustomFunction = func(function string, user interface{}, message messages.Message) (status int, responseBody map[string]interface{}, err *utils.Error) {

	if config.SystemConfig.Functions == nil {
		return
	}
	endpoint := config.SystemConfig.Functions["endpoint"].(string)

	url := endpoint + function
	data := createRequestBody(user, message)
	return sendRequest(url, data)
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
		err = &utils.Error{http.StatusInternalServerError, "Decoding body from hook server failed."}
	}
	return
}

var createRequestBody = func(user interface{}, message messages.Message) (map[string]interface{}) {

	data := make(map[string]interface{})
	data["user"] = user
	data["parameters"] = message.Parameters
	data["body"] = message.Body
	data["multipart"] = message.MultipartForm
	return data
}