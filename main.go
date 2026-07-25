/*
 * Copyright (C) 2026 b247_eu, https://b247.eu.org
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <https://opensource.org/license/gpl-3.0/>.
 */
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

const (
	APIBase   = "https://hidroelectrica-svc.smartcmobile.com"
	APIHost   = "hidroelectrica-svc.smartcmobile.com"
	UserAgent = "iHidro/1 CFNetwork/3860.600.12 Darwin/25.5.0" //"okhttp/4.9.0"
)

type APIResponse struct {
	StatusCode int
	Headers    http.Header
	Body       map[string]interface{}
	RawBody    []byte
}

type Config struct {
	User    string `json:"user"`
	Pass    string `json:"pass"`
	UAN     string `json:"uan"`
	POD     string `json:"pod"`
	Install string `json:"install"`
	Acc     string `json:"acc"`
}

type HidroClient struct {
	Config       Config
	UserID       string
	SessionToken string
	HTTP         *http.Client
}

func BuildUsageEntity(previousRead map[string]interface{}, newMeterRead, newMeterReadDate string) map[string]interface{} {
	entity := make(map[string]interface{})
	for k, v := range previousRead {
		entity[k] = v
	}

	entity["newmeterread"] = newMeterRead
	entity["NewMeterReadDate"] = newMeterReadDate

	return entity
}

func main() {
	submitIndex := flag.String("submitIndex", "", "Submit index value")
	getIndexHistory := flag.Bool("getIndexHistory", false, "Retrieve index history")
	getSelfMeterReadAllowed := flag.Bool("getSelfMeterReadAllowed", false, "Retrieve open window for self meter read")
	authFile := flag.String("authConfig", "auth.json", "Configuration file path")
	flag.Parse()

	if flag.NFlag() == 0 {
		fmt.Println("No flags passed")
		flag.Usage()
		os.Exit(1)
	}

	jar, _ := cookiejar.New(nil)
	client := &HidroClient{
		Config: loadConfig(*authFile),
		HTTP: &http.Client{
			Jar: jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
		},
	}

	_, err := client.Authenticate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authenticate failed: %s\n", err)
		os.Exit(1)
	}

	if *submitIndex != "" {
		_, err := client.SubmitIndex(*submitIndex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SubmitIndex failed: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("SubmitIndex success: %s\n", *submitIndex)
	}

	if *getIndexHistory {
		res, err := client.GetIndexHistory()
		if err != nil {
			fmt.Fprintf(os.Stderr, "GetIndexHistory failed: %s\n", err)
			if res != nil {
				fmt.Fprintf(os.Stderr, "Response: %s\n", string(res.RawBody))
			}
			os.Exit(1)
		}

		var resRawBody map[string]interface{}
		json.Unmarshal(res.RawBody, &resRawBody)
		result := resRawBody["result"].(map[string]interface{})
		data := result["Data"]

		output, _ := json.Marshal(data)
		fmt.Fprintf(os.Stdout, "%s\n", output)

	}

	if *getSelfMeterReadAllowed {
		res, err := client.GetWindowDates()
		if err != nil {
			fmt.Fprintf(os.Stderr, "GetWindowDates failed: %s\n", err)
			os.Exit(1)
		}
		var resRawBody map[string]interface{}
		json.Unmarshal(res.RawBody, &resRawBody)
		result := resRawBody["result"].(map[string]interface{})
		data := result["Data"]

		output, _ := json.Marshal(data)

		fmt.Printf("%s\n", output)
	}

}

func (c *HidroClient) SubmitIndex(val string) (*APIResponse, error) {

	res1, err := c.GetWindowDates()
	if err != nil {
		return res1, fmt.Errorf("%w", err)
	}
	submitIndexOpenWindows := extractData(res1.Body)
	isOpen := fmt.Sprintf("%v", submitIndexOpenWindows["Is_Window_Open"])
	if isOpen == "0" {
		closeDate := submitIndexOpenWindows["ClosingDate"]
		nextOpen := submitIndexOpenWindows["NextMonthOpeningDate"]

		return res1, fmt.Errorf("Submission window closed (%v - %v)",
			nextOpen, closeDate)
	}

	res2, err := c.GetPods()
	if err != nil {
		return res2, err
	}

	res3, err := c.GetPreviousMeterRead()
	if err != nil {
		return res3, err
	}

	res3tMap, _ := res3.Body["result"].(map[string]interface{})
	res3Data, _ := res3tMap["Data"].([]interface{})
	if len(res3Data) == 0 {
		return res3, fmt.Errorf("GetPreviousMeterRead failure in reading result.Data")
	}

	var primingEntities []map[string]interface{}
	for _, item := range res3Data {
		if reading, ok := item.(map[string]interface{}); ok {
			primingEntities = append(primingEntities, BuildUsageEntity(reading, "", ""))
		}
	}
	_, _ = c.GetMeterValue(primingEntities)

	nowStr := time.Now().Format("02-01-2006")
	var finalEntities []map[string]interface{}
	for _, item := range res3Data {
		if reading, ok := item.(map[string]interface{}); ok {
			finalEntities = append(finalEntities, BuildUsageEntity(reading, val, nowStr))
		}
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))

	payload := map[string]interface{}{
		"UserId":                   c.UserID,
		"podValue":                 c.Config.POD,
		"InstallationNumber":       c.Config.Install,
		"AccountNumber":            c.Config.Acc,
		"UsageSelfMeterReadEntity": finalEntities,
	}

	return c.doHttpRequest("POST", "/Service/SelfMeterReading/SubmitSelfMeterRead", "1", auth, payload)
}

func (c *HidroClient) GetIndexHistory() (*APIResponse, error) {

	res1, err := c.GetPods()
	if err != nil {
		return res1, err
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))

	payload := map[string]interface{}{
		"utilityAccountNumber": c.Config.UAN,
		"podValue":             c.Config.POD,
		"LanguageCode":         "RO",
		"InstallationNumber":   c.Config.Install,
		"SerialNumber":         []string{},
	}

	return c.doHttpRequest("POST", "/Service/IndexHistory/GetMeterReadHistory", "1", auth, payload)
}

func (c *HidroClient) Authenticate() (*APIResponse, error) {
	res1, err := c.doHttpRequest("POST", "/API/UserLogin/GetId", "0", "", map[string]interface{}{})
	if err != nil {
		return res1, err
	}

	data1 := extractData(res1.Body)
	if data1 == nil {
		return res1, fmt.Errorf("Missing result Data in GetId response")
	}

	key, _ := data1["key"].(string)
	tokenId, _ := data1["tokenId"].(string)
	message, _ := data1["Message"].(string)
	if key == "" || tokenId == "" {
		return res1, fmt.Errorf("%s", message)
	}

	authPre := "Basic " + base64.StdEncoding.EncodeToString([]byte(key+":"+tokenId))
	payload := map[string]interface{}{
		"deviceType": "MobileApp", "OperatingSystem": "Android", "LanguageCode": "RO",
		"password": c.Config.Pass, "UserId": c.Config.User, "UpdatedDate": time.Now().Format("01/02/2006 15:04:05"),
	}

	res2, err := c.doHttpRequest("POST", "/API/UserLogin/ValidateUserLogin", "0", authPre, payload)
	if err != nil {
		return res2, err
	}

	res2Map, _ := res2.Body["result"].(map[string]interface{})
	message2, _ := res2Map["Message"].(string)

	data2 := extractData(res2.Body)
	table, ok := data2["Table"].([]interface{})

	if !ok || len(table) == 0 {
		return res2, fmt.Errorf("%s", message2)
	}

	for _, entry := range table {
		row := entry.(map[string]interface{})

		getID := func(v interface{}) string {
			if s, ok := v.(string); ok {
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
			return fmt.Sprintf("%v", v)
		}

		uan := getID(row["UtilityAccountNumber"])
		acc := getID(row["AccountNumber"])

		if uan == c.Config.UAN {
			c.UserID = fmt.Sprintf("%v", row["UserID"])
			c.SessionToken = fmt.Sprintf("%v", row["SessionToken"])

			c.Config.UAN = uan
			c.Config.Acc = acc
			return res2, nil
		}
	}
	return res2, fmt.Errorf("UAN %s not found in contract list", c.Config.UAN)

}

func (c *HidroClient) GetWindowDates() (*APIResponse, error) {
	if c.UserID == "" || c.SessionToken == "" {
		return nil, fmt.Errorf("Client not authenticated!")
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))

	payload := map[string]interface{}{
		"MeterType":            "E", // "E" from Electricity
		"UserID":               c.UserID,
		"UtilityAccountNumber": c.Config.UAN,
		"AccountNumber":        c.Config.Acc,
	}

	return c.doHttpRequest("POST", "/Service/SelfMeterReading/GetWindowDates", "1", auth, payload)
}

func (c *HidroClient) GetPods() (*APIResponse, error) {
	if c.UserID == "" || c.SessionToken == "" {
		return nil, fmt.Errorf("Client not authenticated!")
	}

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))

	payload := map[string]interface{}{
		"MeterType":            "E", // Electricity
		"UserID":               c.UserID,
		"UtilityAccountNumber": c.Config.UAN,
		"AccountNumber":        c.Config.Acc,
	}

	res, err := c.doHttpRequest("POST", "/Service/SelfMeterReading/GetPods", "1", auth, payload)
	if err != nil {
		return res, err
	}

	if resMap, ok := res.Body["result"].(map[string]interface{}); ok {
		if dataSlice, ok := resMap["Data"].([]interface{}); ok && len(dataSlice) > 0 {
			firstItem := dataSlice[0].(map[string]interface{})

			c.Config.Install = fmt.Sprintf("%v", firstItem["installation"])
			c.Config.POD = fmt.Sprintf("%v", firstItem["pod"])
		}
	}

	return res, err
}

func (c *HidroClient) GetPreviousMeterRead() (*APIResponse, error) {

	// res1, err := c.GetPods()
	// if err != nil {
	// 	return res1, err
	// }

	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))

	payload := map[string]interface{}{
		"UserID":               c.UserID,
		"UtilityAccountNumber": c.Config.UAN,
		"InstallationNumber":   c.Config.Install,
		"podValue":             c.Config.POD,
		"LanguageCode":         "RO",
		"BasicValue":           "",
		"CustomerNumber":       c.Config.Acc,
		"Distributor":          "",
	}

	return c.doHttpRequest("POST", "/Service/SelfMeterReading/GetPreviousMeterRead", "1", auth, payload)
}

func (c *HidroClient) GetMeterValue(usageEntities []map[string]interface{}) (*APIResponse, error) {
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.UserID+":"+c.SessionToken))
	payload := map[string]interface{}{
		"podValue":                 c.Config.POD,
		"UsageSelfMeterReadEntity": usageEntities,
		"UserId":                   c.UserID,
		"AccountNumber":            c.Config.Acc,
		"InstallationNumber":       c.Config.Install,
	}
	return c.doHttpRequest("POST", "/Service/SelfMeterReading/GetMeterValue", "1", auth, payload)
}

func (c *HidroClient) doHttpRequest(method, endpoint, sourceType, auth string, payload interface{}) (*APIResponse, error) {
	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, APIBase+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Host = APIHost
	req.Header["Content-Type"] = []string{"application/json; charset=utf-8"}
	req.Header["Accept"] = []string{"application/json"}
	req.Header["User-Agent"] = []string{UserAgent}
	req.Header["SourceType"] = []string{sourceType}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	// debug request
	// requestDump, err := httputil.DumpRequestOut(req, true)
	// if err == nil {
	// 	fmt.Printf("--- REQUEST ---\n%s\n\n", string(requestDump))
	// }

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// debug response
	// responseDump, err := httputil.DumpResponse(resp, true)
	// if err == nil {
	// 	fmt.Printf("--- RESPONSE ---\n%s\n\n", string(responseDump))
	// }

	bodyBytes, _ := io.ReadAll(resp.Body)
	var bodyMap map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &bodyMap)

	apiResp := &APIResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyMap,
		RawBody:    bodyBytes,
	}

	if resp.StatusCode >= 400 {
		return apiResp, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return apiResp, nil
}

func extractData(body map[string]interface{}) map[string]interface{} {
	if res, ok := body["result"].(map[string]interface{}); ok {
		if data, ok := res["Data"].(map[string]interface{}); ok {
			return data
		}
	}
	return nil
}

func loadConfig(path string) Config {
	var c Config
	d, _ := os.ReadFile(path)
	_ = json.Unmarshal(d, &c)
	if v := os.Getenv("HIDRO_USER"); v != "" {
		c.User = v
	}
	if v := os.Getenv("HIDRO_PASS"); v != "" {
		c.Pass = v
	}
	if v := os.Getenv("HIDRO_UAN"); v != "" {
		c.UAN = v
	}
	return c
}
