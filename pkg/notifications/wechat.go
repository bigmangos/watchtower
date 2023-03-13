package notifications

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	t "github.com/containrrr/watchtower/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	getTokenUrl = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	sendMsgUrl  = "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token="
)

type wechatNotifier struct {
	corpid     string
	corpsecret string
	toUser     string
	agentid    string
}

type GetTokenRsp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type SendMspRsp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func newWechatNotifier(cmd *cobra.Command) *wechatNotifier {
	flags := cmd.Flags()

	params, _ := flags.GetString("notification-wechat-params")
	if len(params) <= 0 {
		log.Errorf("Required argument --notification-wechat-params(cli) or WATCHTOWER_NOTIFICATION_WECHAT_PARAMS(env) is empty.")
		return nil
	}

	allParas := strings.Split(params, ",")
	if len(allParas) < 4 {
		log.Fatal("Required argument --notification-wechat(cli) or WATCHTOWER_NOTIFICATION_WECHAT(env) is invalid.")
		return nil
	}

	return &wechatNotifier{
		corpid:     allParas[0],
		corpsecret: allParas[1],
		toUser:     allParas[2],
		agentid:    allParas[3],
	}
}

func (w *wechatNotifier) SendMsg(msg string) {
	if w == nil {
		return
	}

	data := map[string]interface{}{
		"touser":  w.toUser,
		"msgtype": "text",
		"agentid": w.agentid,
		"text": map[string]string{
			"content": msg,
		},
		"safe": "0",
	}

	requestBody, err := json.Marshal(&data)
	if err != nil {
		log.Fatal("SendMsg marshal send msg body err: ", err)
		return
	}

	responseBody, err := httpPost(sendMsgUrl+w.getAccessToken(), requestBody)
	if err != nil {
		log.Fatal("SendMsg httpPost err: ", err)
		return
	}

	var rsp SendMspRsp
	if err = json.Unmarshal(responseBody, &rsp); err != nil {
		log.Fatal("SendMsg Unmarshal err: ", err)
		return
	}

	if strings.Contains(rsp.ErrMsg, "ok") {
		log.Error("SendMsg success")
		return
	}

	log.Fatal("SendMsg err: ", rsp.ErrMsg)
}

func (w *wechatNotifier) getAccessToken() string {
	data := map[string]interface{}{
		"corpid":     w.corpid,
		"corpsecret": w.corpsecret,
	}

	requestBody, err := json.Marshal(&data)
	if err != nil {
		log.Fatal("marshal get access token body err: ", err)
		return ""
	}

	responseBody, err := httpPost(getTokenUrl, requestBody)
	if err != nil {
		log.Fatal("getAccessToken httpPost err: ", err)
		return ""
	}

	var rsp GetTokenRsp
	if err = json.Unmarshal(responseBody, &rsp); err != nil {
		log.Fatal("Unmarshal err: ", err)
		return ""
	}

	return rsp.AccessToken
}

func (w *wechatNotifier) generateWechatMsg(data StaticData, reportData t.Report) string {
	if w == nil {
		return ""
	}

	var report jsonMap
	if reportData != nil {
		report = jsonMap{
			`scanned`: marshalReports(reportData.Scanned()),
			`updated`: marshalReports(reportData.Updated()),
			`failed`:  marshalReports(reportData.Failed()),
			`skipped`: marshalReports(reportData.Skipped()),
			`stale`:   marshalReports(reportData.Stale()),
			`fresh`:   marshalReports(reportData.Fresh()),
		}
	}

	jsonData, err := json.Marshal(jsonMap{
		`report`: report,
		`title`:  data.Title,
		`host`:   data.Host,
	})
	if err != nil {
		log.Fatal("generateWechatMsg marshal err: ", err)
		return ""
	}

	return string(jsonData)
}

func httpPost(url string, data []byte) ([]byte, error) {
	client := &http.Client{}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Fatal("NewRequest err: ", err)
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody := new(bytes.Buffer)
	_, err = responseBody.ReadFrom(response.Body)
	if err != nil {
		return nil, err
	}

	return responseBody.Bytes(), nil
}
