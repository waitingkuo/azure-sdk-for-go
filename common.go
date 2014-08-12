package azureSdkForGo

import (
	"fmt"
	"os"
	"io"
	"bytes"
	"time"
	"strings"
	"errors"
	"os/exec"
	"encoding/xml"
	"github.com/MSOpenTech/azure-sdk-for-go/core/tls"
	"github.com/MSOpenTech/azure-sdk-for-go/core/http"
)

func PrintErrorAndExit(err error) {
	fmt.Println("Error: ")
	fmt.Println(err)
	os.Exit(2)
}

func SendAzureGetRequest(url string) ([]byte, error){
	response, err := SendAzureRequest(url, "GET", nil)
	if err != nil {
		return nil, err
	}

	responseContent := getResponseBody(response)
	return responseContent, nil
}

func SendAzurePostRequest(url string, data []byte) (string, error){
	response, err := SendAzureRequest(url, "POST", data)
	if err != nil {
		return "", err
	}

	requestId := response.Header["X-Ms-Request-Id"]
	return requestId[0], nil
}

func SendAzureRequest(url string, requestType string,  data []byte) (*http.Response, error){
	client := createHttpClient()

	request, reqErr := createAzureRequest(url, requestType, data)
	if reqErr != nil {
		return nil, reqErr
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode > 299 {
		responseContent := getResponseBody(response)
		error := getAzureError(responseContent)
		return nil, error
	}

	return response, nil
}

func ExecuteCommand(command string) ([]byte, error) {
	parts := strings.Fields(command)
	head := parts[0]
	parts = parts[1:len(parts)]

	cmd := exec.Command(head, parts...)

	out, err := cmd.Output()

	if err != nil {
		return nil, err
	}

	return out, nil
}

func GetOperationStatus(operationId string) (*Operation, error){
	operation := new(Operation)
	url := "operations/" + operationId
	response, azureErr := SendAzureGetRequest(url)
	if azureErr != nil {
		return nil, azureErr
	}

	err := xml.Unmarshal(response, operation)
	if err != nil {
		return nil, err
	}

	return operation, nil
}

func WaitAsyncOperation(operationId string) (error) {
	status := "InProgress"
	operation := new(Operation)
	err := errors.New("")
	for status == "InProgress" {
		time.Sleep(2000 * time.Millisecond)
		operation, err = GetOperationStatus(operationId)
		if err != nil {
			return err
		}

		status = operation.Status
	}

	if status == "Failed" {
		return errors.New(operation.Error.Message)
	}

	return nil
}

func getAzureError(responseBody []byte) (error){
	error := new(AzureError)
	err := xml.Unmarshal(responseBody, error)
	if err != nil {
		return err
	}

	return error
}

func createAzureRequest(url string, requestType string,  data []byte) (*http.Request, error){
	var request *http.Request
	var err error

	url = fmt.Sprintf("https://management.core.windows.net/%s/" + url, GetPublishSettings().SubscriptionID)
	if data != nil {
		body := bytes.NewBuffer(data)
		request, err = http.NewRequest(requestType, url, body)
	} else {
		request, err = http.NewRequest(requestType, url, nil)
	}

	if err != nil {
		return nil, err
	}

	request.Header.Add("x-ms-version", "2014-05-01")
	request.Header.Add("Content-Type", "application/xml")

	return request, nil
}

func createHttpClient() (*http.Client){
	cert, _ := tls.LoadX509KeyPair(GetPublishSettings().SubscriptionCert, GetPublishSettings().SubscriptionCert)

	ssl := &tls.Config{}
	ssl.Certificates = []tls.Certificate{cert}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: ssl,
		},
	}

	return client
}

func getResponseBody(response *http.Response) ([]byte){

	responseBody := make([]byte, response.ContentLength)
	io.ReadFull(response.Body, responseBody)
	return responseBody
}

type AzureError struct {
	XMLName   			xml.Name `xml:"Error"`
	Code				string
	Message				string
}

func (e *AzureError) Error() string {
	return fmt.Sprintf("Code: %s, Message: %s", e.Code, e.Message)
}

type Operation struct {
	XMLName   			xml.Name `xml:"Operation"`
	ID					string
	Status				string
	HttpStatusCode		string
	Error 				AzureError
}