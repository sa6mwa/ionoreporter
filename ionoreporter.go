/* ionoreporter.go
 * Copyright 2019 SA6MWA Michel <sa6mwa@radiohorisont.se>
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
  log "github.com/sirupsen/logrus"
  "github.com/jung-kurt/gofpdf"
)

/* output directory where ionoreports-TIMESTAMP.pdf will be saved,
 * can be changed by OUTDIR environment variable */
var outputDirectory = "."

/* download new ionogram every interval minutes */
var interval = time.Duration(15 * time.Minute)

/* urls of ionograms, see gofpdf.New below for hardcoded usage that you need to
 * change if you change to other ionograms or images */
var urls = []map[string]string{{
  "name": "juliusruh",
  "url": "https://www.iap-kborn.de/fileadmin/user_upload/MAIN-abteilung/radar/Radars/Ionosonde/Plots/LATEST.PNG",
  "format": "png",
  }, {
  "name": "tromso",
  "url": "http://www.tgo.uit.no/ionosonde/latest.gif",
  "format": "gif",
  }, {
  "name": "kiruna",
  "url": "http://www2.irf.se/ionogram/dynasonde_kir/sao/latest.gif",
  "format": "gif",
  }, {
  "name": "lycksele",
  "url": "http://www2.irf.se/ionogram/plots/ionoLy.gif",
  "format": "gif",
}}


/* version gets replaced build-time by go build -ldflags, see Makefile for more info */
var version = "1.0.1"


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
  for _, is := range urls {
    log.Infof("Downloading %s", is["url"])
    if _, err := downloadFile(is["url"], is["format"], is["name"]); err != nil {
      log.Errorf("Error downloading %s: %s", is["url"], err)
    }
  }
  for k, v := range outputs {
    log.Infof("%s ionogram saved as %s", k, v)
  }

  pdfFileName := outputDirectory + "/" + "ionoreport-" + time.Now().UTC().Format("20060102T150405") + ".pdf"

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
  cleanup()
}



/* main() */
func main() {
  log.SetFormatter(UTCFormatter{&log.TextFormatter{
    FullTimestamp: true,
  }})
  if outdir, ok := os.LookupEnv("OUTDIR"); ok {
    outputDirectory = outdir
  }
  log.Infof("Starting ionoreporter %s, OUTDIR == %s", version, outputDirectory)
  for {
    ionogramDownloader()
    log.Infof("Waiting %s until next run", interval.String())
    time.Sleep(interval)
  }
}

