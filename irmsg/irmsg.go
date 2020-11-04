package irmsg

import (
  "fmt"
  "bytes"
  "strings"
  "encoding/json"
  "net/http"
  "crypto/tls"
  "time"
)

func PostJson(webhookUrl string, jsonBytes []byte, okresponse string) error {
  if len(webhookUrl) == 0 {
    return fmt.Errorf("Webhook URL empty")
  }
  req, err := http.NewRequest(http.MethodPost, webhookUrl, bytes.NewBuffer(jsonBytes))
  if err != nil {
    return nil
  }
  req.Header.Add("Content-type", "application/json")
  tr := &http.Transport{
    TLSClientConfig: &tls.Config{ InsecureSkipVerify: true },
  }
  client := &http.Client{
    Timeout: 10 * time.Second,
    Transport: tr,
  }
  resp, err := client.Do(req)
  if err != nil {
    return err
  }
  defer resp.Body.Close()
  buf := new(bytes.Buffer)
  buf.ReadFrom(resp.Body)
  if len(buf.String()) > 0 && len(okresponse) > 0 {
    if ! strings.HasPrefix(buf.String(), okresponse) {
      return fmt.Errorf("Non-ok response returned")
    }
  } else {
    if resp.StatusCode < 200 || resp.StatusCode > 299 {
      return fmt.Errorf("Got return code %d", resp.StatusCode)
    }
  }
  return nil
}

type slackRequestBody struct {
  Text string `json:"text"`
  Blocks []slackBlock `json:"blocks"`
}
type slackBlock struct {
  Type string `json:"type"`
  Text slackBlockText `json:"text"`
}
type slackBlockText struct {
  Type string `json:"type"`
  Text string `json:"text"`
}
func SendSlackMsg(webhookUrl, header, markdown string) error {
  slackBody, _ := json.Marshal(slackRequestBody{
    Text: header,
    Blocks: []slackBlock{
      {
        Type: "section",
        Text: slackBlockText{
          Type: "mrkdwn",
          Text: markdown,
        },
      },
    },
  })
  err := PostJson(webhookUrl, slackBody, "ok")
  if err != nil {
    return err
  }
  return nil
}

type discordRequestBody struct {
  Content string `json:"content"`
}
func SendDiscordMsg(webhookUrl, content string) error {
  discordBody, _ := json.Marshal(discordRequestBody{
    Content: content,
  })
  err := PostJson(webhookUrl, discordBody, `{"id":`)
  if err != nil {
    return err
  }
  return nil
}

