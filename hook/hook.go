package hook
import (
	"github.com/eluleci/dock/config"
	"github.com/eluleci/dock/utils"
	"net/http"
	"strings"
	"github.com/eluleci/dock/messages"
	"bytes"
	"encoding/json"
)

var SendHookRequest = func(className, when, method string, user interface{}, message messages.Message) (status int, responseBody interface{}, err *utils.Error) {

	// TODO: check map[string]interface{} before casting

	if config.SystemConfig.WebHook == nil {
		return
	}
	endpoint := config.SystemConfig.WebHook["endpoint"].(string)

	methods, hasMethods := config.SystemConfig.WebHook["methods"]
	if !hasMethods {
		err = &utils.Error{http.StatusInternalServerError, "Web hook methods are not defined in server configuration."}
		return
	}
	methodsAsMap := methods.(map[string]interface{})

	hooksForClass, hasHooksForClass := methodsAsMap[className]
	if !hasHooksForClass {
		return
	}
	hooksForClassAsMap := hooksForClass.(map[string]interface{})

	providedMethodsForClass, hasProvidedMethodsForClass := hooksForClassAsMap[when]
	if !hasProvidedMethodsForClass {
		return
	}
	providedMethodsForClassAsMap := providedMethodsForClass.(map[string]interface{})

	endpointForMethod, hasEndpointForMethod := providedMethodsForClassAsMap[strings.ToLower(method)]
	if !hasEndpointForMethod {
		return
	}

	url := endpoint + endpointForMethod.(string)
	data := createHookRequestBody(user, message)
	return sendHookRequest(url, data)
}

var sendHookRequest = func(url string, body interface{}) (status int, responseBody interface{}, err *utils.Error) {

	bodyAsBytes, encodeErr := json.Marshal(body)
	if encodeErr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Encoding hook request data failed."}
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
	responseBody = resp.Body
	return
}

var createHookRequestBody = func(user interface{}, message messages.Message) (map[string]interface{}) {

	data := make(map[string]interface{})
	data["user"] = user
	data["parameters"] = message.Parameters
	data["body"] = message.Body
	data["multipart"] = message.MultipartForm
	return data
}