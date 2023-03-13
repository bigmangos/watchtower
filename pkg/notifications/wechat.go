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
		log.Error("Required argument --notification-wechat-params(cli) or WATCHTOWER_NOTIFICATION_WECHAT_PARAMS(env) is empty.")
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
		log.Errorf("SendMsg marshal send msg body err: %v", err)
		return
	}

	responseBody, err := httpPost(sendMsgUrl+w.getAccessToken(), requestBody)
	if err != nil {
		log.Errorf("SendMsg httpPost err: %v", err)
		return
	}

	var rsp SendMspRsp
	if err = json.Unmarshal(responseBody, &rsp); err != nil {
		log.Errorf("SendMsg Unmarshal err: %v", err)
		return
	}

	if strings.Contains(rsp.ErrMsg, "ok") {
		log.Info("SendMsg success")
		return
	}

	log.Errorf("SendMsg err: %v", rsp.ErrMsg)
}

func (w *wechatNotifier) getAccessToken() string {
	data := map[string]interface{}{
		"corpid":     w.corpid,
		"corpsecret": w.corpsecret,
	}

	requestBody, err := json.Marshal(&data)
	if err != nil {
		log.Error("marshal get access token body err: ", err)
		return ""
	}

	responseBody, err := httpPost(getTokenUrl, requestBody)
	if err != nil {
		log.Error("getAccessToken httpPost err: ", err)
		return ""
	}

	var rsp GetTokenRsp
	if err = json.Unmarshal(responseBody, &rsp); err != nil {
		log.Error("Unmarshal err: ", err)
		return ""
	}

	return rsp.AccessToken
}

func (w *wechatNotifier) generateWechatMsg(data StaticData, reportData t.Report) string {
	if w == nil {
		log.Info("wechatNotifier is nil")
		return ""
	}

	if reportData == nil {
		log.Info("reportData is nil")
		return data.Title + "\n\n" + "启动成功..."
	}

	updates := reportData.Updated()
	fails := reportData.Failed()
	if len(updates) == 0 && len(fails) == 0 {
		log.Info("updates and fails are nil")
		return ""
	}

	var buf strings.Builder
	buf.WriteString(data.Title + "\n\n")
	if len(updates) > 0 {
		buf.WriteString("已更新: " + "\n")
		for _, update := range updates {
			buf.WriteString(strings.Replace(update.Name(), "/", "", -1) + " + " + update.ImageName() + "\n")
		}
		buf.WriteString("\n")
	}

	if len(fails) > 0 {
		buf.WriteString("更新失败: " + "\n")
		for _, fail := range fails {
			buf.WriteString(strings.ReplaceAll(fail.Name(), "/", "") + " + " + fail.ImageName() + "\n")
		}
		buf.WriteString("\n")
	}

	freshs := reportData.Fresh()
	if len(freshs) > 0 {
		buf.WriteString("已刷新: " + "\n")
		for _, fresh := range freshs {
			buf.WriteString(strings.ReplaceAll(fresh.Name(), "/", "") + " + " + fresh.ImageName() + "\n")
		}
		buf.WriteString("\n")
	}

	skips := reportData.Skipped()
	if len(skips) > 0 {
		buf.WriteString("已跳过: " + "\n")
		for _, skip := range skips {
			buf.WriteString(strings.ReplaceAll(skip.Name(), "/", "") + " + " + skip.ImageName() + "\n")
		}
		buf.WriteString("\n")
	}

	stales := reportData.Stale()
	if len(stales) > 0 {
		buf.WriteString("正在跟踪: " + "\n")
		for _, stale := range stales {
			buf.WriteString(strings.ReplaceAll(stale.Name(), "/", "") + " + " + stale.ImageName() + "\n")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

func httpPost(url string, data []byte) ([]byte, error) {
	client := &http.Client{}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
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
