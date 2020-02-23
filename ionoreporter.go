/* ionoreporter.go
 * Copyright 2019,2020 SA6MWA Michel <sa6mwa@radiohorisont.se>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main
import (
  "io"
  "io/ioutil"
  "time"
  "errors"
  "net/http"
  "crypto/tls"
  "os"
  "fmt"
  "image"
  _ "image/jpeg"
  "image/png"
  _ "image/gif"
  "bytes"
  "strings"
  "strconv"
  "encoding/json"

  log "github.com/sirupsen/logrus"
  "github.com/jung-kurt/gofpdf"
  "github.com/kelseyhightower/envconfig"
  "github.com/oliamb/cutter"
  "github.com/otiai10/gosseract"
)

type Config struct {
  Urls []Ionosonde `ignored:"true"`
  SlackWebhookUrl string `envconfig:"SLACKURL"`
  Slack bool `envconfig:"SLACK"`
  PushFrequency int `envconfig:"PUSH_FREQUENCY"`
  Interval time.Duration `envconfig:"INTERVAL"`
  OutDir string `envconfig:"OUTDIR"`
}
type Ionosonde struct {
  Name string
  Url string
  Format string
}
const (
  FormatPng string = "png"
  FormatGif string = "gif"
)

var cnf = &Config{
  Urls: []Ionosonde{
    {
      Name: "juliusruh",
      Url: "https://www.iap-kborn.de/fileadmin/user_upload/MAIN-abteilung/radar/Radars/Ionosonde/Plots/LATEST.PNG",
      Format: FormatPng,
    }, {
      Name: "tromso",
      Url: "http://www.tgo.uit.no/ionosonde/latest.gif",
      Format: FormatGif,
    }, {
      Name: "kiruna",
      Url: "http://www2.irf.se/ionogram/dynasonde_kir/sao/latest.gif",
      Format: FormatGif,
    }, {
      Name: "lycksele",
      Url: "http://www2.irf.se/ionogram/plots/ionoLy.gif",
      Format: FormatGif,
    },
  },
  OutDir: ".", // default output directory, change with env var IRPT_OUTDIR
  Interval: time.Duration(15 * time.Minute), // default time period, change with env var IRPT_INTERVAL
  PushFrequency: 2, // send slack notification by default every 15*2 minutes (30 minutes)
}


/* version gets replaced build-time by go build -ldflags, see Makefile for more info */
var version = "2.0.0"



type IonogramParameters struct {
  Name string
  Date string
  FoF2 float32
  FoE float32
  FxI float32
  HmE float32
  HmF2 float32
  Muf100 float32
  Muf200 float32
  Muf400 float32
  Muf600 float32
  Muf800 float32
  Muf1000 float32
}

func (ip IonogramParameters) String(newline bool, full bool) string {
  date := "UNKNOWN"
  if len(ip.Date) != 0 {
    date = ip.Date
  }
  out := ""
  out += ipstr("foF2", ip.FoF2, out)
  out += ipstr("85%foF2", ip.FoF2 * 0.85, out)
  out2 := ""
  out2 += ipstr("foE", ip.FoE, out2)
  out2 += ipstr("fxI", ip.FxI, out2)
  out2 += ipstr("hmE", ip.HmE, out2)
  out2 += ipstr("hmF2", ip.HmF2, out2)
  out3 := ""
  out3 += ipstr("MUF100", ip.Muf100, out3)
  out3 += ipstr("MUF200", ip.Muf200, out3)
  out3 += ipstr("MUF400", ip.Muf400, out3)
  out3 += ipstr("MUF600", ip.Muf600, out3)
  out3 += ipstr("MUF800", ip.Muf800, out3)
  out3 += ipstr("MUF1Mm", ip.Muf1000, out3)
  str := ""
  if len(out) < 1 {
    return "No foF2 to report!"
  }
  if len(ip.Name) > 0 {
    str = ip.Name + " "
  }
  if newline {
    str += date + "\n" + out + "\n"
    if len(out2) > 0 {
      str += out2 + "\n"
    }
    if len(out3) > 0 && full {
      str += out3 + "\n"
    }
  } else {
    str += date + ": " + out
    if len(out2) > 0 {
      str += ", " + out2
    }
    if len(out3) > 0 && full {
      str += ", " + out3
    }
  }
  return str
}





func getText(img *bytes.Buffer) (string) {
  client := gosseract.NewClient()
  defer client.Close()
  client.SetImageFromBytes(img.Bytes())
  text, err := client.Text()
  if err != nil {
    return ""
  }
  return strings.TrimSpace(text)
}

// getText from part of image
func getTextFromCut(img image.Image, width, height, x, y int) (string, error) {
  crop, err := cutter.Crop(img, cutter.Config{
    Mode: cutter.TopLeft,
    Width: width,
    Height: height,
    Anchor: image.Point{x, y},
  })
  if err != nil {
    return "", err
  }
  buf := new(bytes.Buffer)
  defer buf.Reset()
  err = png.Encode(buf, crop)
  if err != nil {
    return "", err
  }
  return getText(buf), nil
}
func getTextFromCutFloat32(img image.Image, width, height, x, y int) (float32, error) {
  crop, err := cutter.Crop(img, cutter.Config{
    Mode: cutter.TopLeft,
    Width: width,
    Height: height,
    Anchor: image.Point{x, y},
  })
  if err != nil {
    return 0.0, err
  }
  buf := new(bytes.Buffer)
  defer buf.Reset()
  err = png.Encode(buf, crop)
  if err != nil {
    return 0.0, err
  }
  text := getText(buf)
  num, err := strconv.ParseFloat(text, 32)
  if err != nil {
    return 0.0, err
  }
  return float32(num), nil
}


// https://www.iap-kborn.de/fileadmin/user_upload/MAIN-abteilung/radar/Radars/Ionosonde/Plots/LATEST.PNG
// http://www.tgo.uit.no/ionosonde/latest.gif
func getParametersFromJuliusruh() (IonogramParameters, error) {
  ip := IonogramParameters{}
  imgf, err := getOutput("juliusruh")
  if err != nil {
    log.Errorf("Ionogram from Juliusruh not available!")
    return ip, err
  }
  reader, err := os.Open(imgf)
  if err != nil {
    log.Errorf("Unable to open juliusruh ionogram: %v", err)
    return ip, err
  }
  defer reader.Close()
  img, _, err := image.Decode(reader)
  if err != nil {
    return ip, err
  }
  //getTextFromCut(img image.Image, width, height, x, y int) (string, error)
  ip.Name = "Juliusruh"
  ip.Date, _ = getTextFromCut(img, 195, 17, 222, 29)
  ip.FoF2, _ = getTextFromCutFloat32(img, 90, 15, 36, 50)
  ip.FoE, _ = getTextFromCutFloat32(img, 101, 16, 27, 98)
  ip.FxI, _ = getTextFromCutFloat32(img, 98, 17, 27, 129)
  ip.HmE, _ = getTextFromCutFloat32(img, 100, 17, 27, 345)
  ip.HmF2, _ = getTextFromCutFloat32(img, 91, 17, 37, 313)
  ip.Muf100, _ = getTextFromCutFloat32(img, 44, 16, 33, 570)
  ip.Muf200, _ = getTextFromCutFloat32(img, 40, 15, 76, 570)
  ip.Muf400, _ = getTextFromCutFloat32(img, 41, 15, 115, 570)
  ip.Muf600, _ = getTextFromCutFloat32(img, 42, 15, 154, 570)
  ip.Muf800, _ = getTextFromCutFloat32(img, 41, 15, 195, 570)
  ip.Muf1000, _ = getTextFromCutFloat32(img, 41, 15, 235, 570)
  return ip, nil
}


func ipstr(name string, val float32, orig string) string {
  if val != 0 {
    str := ""
    if len(orig) != 0 {
      str = ", "
    }
    return str + name + "=" + fmt.Sprintf("%.2f", val)
  }
  return ""
}





var ionogramParametersCounter int = 0
var accumulatedIonogramParameters = []IonogramParameters{}
func accumulateIonogramParametersFromJuliusruh() (IonogramParameters, error) {
  ip, err := getParametersFromJuliusruh()
  if err != nil {
    return ip, err
  }
  accumulatedIonogramParameters = append(accumulatedIonogramParameters, ip)
  ionogramParametersCounter++
  log.Info(ip.String(false, true))
  return ip, nil
}
func resetAccumulatedIonogramParameters() {
  ionogramParametersCounter = 0
  accumulatedIonogramParameters = []IonogramParameters{}
}


func composeSlackMessage() (string, string) {
  header := "Ionizer says..."
  message := ""
  if len(accumulatedIonogramParameters) < 1 {
    log.Warningf("composeSlackMessage(): accumulatedIonogramParameters is empty!")
    return "", ""
  }
  latestIp := accumulatedIonogramParameters[len(accumulatedIonogramParameters)-1]
  var latest []string
  if latestIp.FoF2 != 0 {
    latest = append(latest, fmt.Sprintf("foF2=%.2f", latestIp.FoF2))
  }
  if latestIp.FxI != 0 {
    latest = append(latest, fmt.Sprintf("fxI=%.2f", latestIp.FxI))
  }
  if latestIp.HmF2 != 0 {
    latest = append(latest, fmt.Sprintf("hmF2=%.1fkm", latestIp.HmF2))
  }
  if len(latest) > 0 {
    header = strings.Join(latest, ", ")
  }
  message = "```\n"
  for _, ip := range accumulatedIonogramParameters {
    message += ip.String(true, false)
  }
  message += "```\n"
  return header, message
}


func slackIonogramParameters() error {
  if ionogramParametersCounter >= cnf.PushFrequency {
    log.Info("Pushing ionogram parameters to Slack")
    slackHeader, slackMarkdown := composeSlackMessage()
    go func() {
      if slackHeader != "" && slackMarkdown != "" {
        err := sendSlackNotification(cnf.SlackWebhookUrl, slackHeader, slackMarkdown)
        if err != nil {
          log.Errorf("Unable to send slack notification: %v", err)
        }
      } else {
        log.Warning("Nothing to send to Slack")
      }
    }()
    resetAccumulatedIonogramParameters()
  } else {
    left := cnf.PushFrequency - ionogramParametersCounter
    plural := ""
    if left > 1 {
      plural = "s"
    }
    log.Infof("%d iteration%s left before pushing to Slack", left, plural)
  }
  return nil
}




var outputs = map[string]string{}
func downloadFile(url string, extension string, tag string) (string, error) {
  tr := &http.Transport{
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
  }
  client := &http.Client{Transport: tr}
  resp, err := client.Get(url)
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()
  out, err := ioutil.TempFile("", "ionoreporter-")
  if err != nil {
    return "", err
  }
  _, err = io.Copy(out, resp.Body)
  out.Close()
  if err != nil {
    os.Remove(out.Name())
    return "", err
  }
  newOutFile := out.Name() + "." + extension
  if err := os.Rename(out.Name(), newOutFile); err != nil {
    os.Remove(out.Name())
    return "", err
  }
  fil, err := os.Stat(newOutFile)
  if err != nil {
    os.Remove(newOutFile)
    return "", err
  }
  if fil.Size() <= 1000 {
    os.Remove(newOutFile)
    return "", errors.New("File is too small to be true")
  }
  outputs[tag] = newOutFile
  return newOutFile, nil
}


func getOutput(name string) (string, error) {
  for tag, imgfile := range outputs {
    if tag == name {
      if _, err := os.Stat(imgfile); err != nil {
        return "", err
      } else {
        return imgfile, nil
      }
    }
  }
  return "", errors.New("Name or file not found")
}



/* https://stackoverflow.com/a/40502637 */
type UTCFormatter struct {
  log.Formatter
}
func (u UTCFormatter) Format(e *log.Entry) ([]byte, error) {
  e.Time = e.Time.UTC()
  return u.Formatter.Format(e)
}


func cleanup() {
  for _, v := range outputs {
    os.Remove(v)
  }
  outputs = map[string]string{}
}


func ionogramDownloader() {
  for _, is := range cnf.Urls {
    log.Infof("Downloading %s", is.Url)
    if _, err := downloadFile(is.Url, is.Format, is.Name); err != nil {
      log.Errorf("Error downloading %s: %s", is.Url, err)
    }
  }
  for k, v := range outputs {
    log.Infof("%s ionogram saved as %s", k, v)
  }

  pdfFileName := cnf.OutDir + "/" + "ionoreport-" + time.Now().UTC().Format("20060102T150405") + ".pdf"

  pdf := gofpdf.New("P","mm","A4","")
  pdf.AddPage()
  pdf.SetFont("Arial", "B", 12)

  if imgf, err := getOutput("juliusruh"); err == nil {
    pdf.Image(imgf, 30, 20, 150, 0, false, "", 0, "")
  } else {
    pdf.Text(60, 80, "Ionogram from Juliusruh not available!")
  }
  if imgf, err := getOutput("tromso"); err == nil {
    pdf.Image(imgf, 30, 152, 150, 0, false, "", 0, "")
  } else {
    pdf.Text(60, 200, "Ionogram from Tromso not available!")
  }
  pdf.WriteAligned(0, 5, "IONOREPORT DE SA6MWA", "C")
  pdf.Ln(5)
  pdf.WriteAligned(0, 5, time.Now().UTC().Format(time.RFC3339), "C")

  pdf.AddPage()

  if imgf, err := getOutput("kiruna"); err == nil {
    pdf.Image(imgf, 35, 10, 140, 0, false, "", 0, "")
  } else {
    pdf.Text(60, 80, "Ionogram from Kiruna not available!")
  }
  if imgf, err := getOutput("lycksele"); err == nil {
    pdf.Image(imgf, 43, 195, 120, 0, false, "", 0, "")
  } else {
    pdf.Text(60, 220, "Ionogram from Lycksele not available!")
  }
  log.Infof("Saving %s", pdfFileName)
  err := pdf.OutputFileAndClose(pdfFileName)
  if err != nil {
    log.Error(err)
  }
}



/* slack.go */
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
// usage:
// webhookUrl := https://hooks.slack.com/services/xxxxx/aaaa/abc123
// err := SendSlackNotification(webhookUrl, "Testing, testing..")
func sendSlackNotification(webhookUrl, header, markdown string) error {
  //slackBody, _ := json.Marshal(slackRequestBody{Text: msg})
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
  req, err := http.NewRequest(http.MethodPost, webhookUrl, bytes.NewBuffer(slackBody))
  if err != nil {
    return err
  }
  req.Header.Add("Content-type", "application/json")
  tr := &http.Transport{
    TLSClientConfig: &tls.Config{ InsecureSkipVerify: true },
  }
  client := &http.Client{Timeout: 10 * time.Second, Transport: tr}
  resp, err := client.Do(req)
  if err != nil {
    return err
  }
  defer resp.Body.Close()
  buf := new(bytes.Buffer)
  buf.ReadFrom(resp.Body)
  if buf.String() != "ok" {
    return errors.New("Non-ok response returned from Slack")
  }
  return nil
}





/* main() */
func main() {
  log.SetFormatter(UTCFormatter{&log.TextFormatter{
    FullTimestamp: true,
  }})
  err := envconfig.Process("IRPT", cnf)
  if err != nil {
    log.Fatalf("envconfig.Process failed: %v", err)
  }

  if cnf.Slack && cnf.SlackWebhookUrl == "" {
    log.Fatalf("Your Slack webhook url (https://hooks.slack.com/services/...) is not configured, configure with environment variable IRPT_SLACKURL")
  }

  log.Infof("Starting ionoreporter %s, IRPT_OUTDIR == %s", version, cnf.OutDir)
  for {
    ionogramDownloader()
    if cnf.Slack {
      accumulateIonogramParametersFromJuliusruh()
      slackIonogramParameters()
    }
    cleanup()
    log.Infof("Waiting %s until next run", cnf.Interval.String())
    time.Sleep(cnf.Interval)
  }
}

