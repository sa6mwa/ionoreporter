/* ionoreporter.go
 * Copyright 2020 SA6MWA Michel <sa6mwa@radiohorisont.se>
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

/*
TODO:
https://tutorialedge.net/golang/writing-a-twitter-bot-golang/
*/


package main
import (
  "io"
  "io/ioutil"
  "time"
  "math/rand"
  "errors"
  "net/http"
  "crypto/tls"
  "os"
  "fmt"
  "sync"
  "image"
  _ "image/jpeg"
  "image/png"
  _ "image/gif"
  "bytes"
  "strings"
  "strconv"
  "regexp"
  "database/sql"

  log "github.com/sirupsen/logrus"
  "github.com/kelseyhightower/envconfig"
  _ "github.com/mattn/go-sqlite3"
  "github.com/oliamb/cutter"
  "github.com/otiai10/gosseract"
  cron "github.com/robfig/cron/v3"
  "github.com/disintegration/gift"
  "github.com/sixdouglas/suncalc"

  "github.com/sa6mwa/ionoreporter/ionizedb"
  "github.com/sa6mwa/ionoreporter/irmsg"
)

/* version gets replaced build-time by go build -ldflags, see Makefile for more info */
var (
  version = "3.1.7"
  mu sync.Mutex
)

const (
  SqliteDateFormat string = "2006-01-02 15:04:05"
  DTGFormat string = "021504ZJan06"
  HourMinute string = "1504"
  Hour string = "15"
  FormatPng string = "png"
  FormatGif string = "gif"
)

type Config struct {
  DatabaseFile string `envconfig:"DBFILE"`
  DiscordDailyWebhookUrl string `envconfig:"DAILY_DISCORDURL"`
  DiscordFrequentWebhookUrl string `envconfig:"FREQUENT_DISCORDURL"`
  SlackDailyWebhookUrl string `envconfig:"DAILY_SLACKURL"`
  SlackFrequentWebhookUrl string `envconfig:"FREQUENT_SLACKURL"`
  Discord bool `envconfig:"DISCORD"`
  Slack bool `envconfig:"SLACK"`
  Daily bool `envconfig:"DAILY"`
  Frequent bool `envconfig:"FREQUENT"`
  DailyReportCronSpec string `envconfig:"DAILY_CRONSPEC"`
  FrequentReportCronSpec string `envconfig:"FREQUENT_CRONSPEC"`
  ScrapeCronSpec string `envconfig:"SCRAPE_CRONSPEC"`
  ScrapeTimeout time.Duration `envconfig:"SCRAPE_TIMEOUT"`
}

var cnf = &Config{
  DatabaseFile: "ionoreporter.db",
  Discord: false,   // do not push reports to discord webhookurl per default
  Slack: false,     // do not push reports to slack webhookurl per default
  Daily: false,     // do not push daily reports to slack or discord per default
  Frequent: false,  // do not post frequent foF2, QSOQRG, etc reports to discord or slack per default
  DailyReportCronSpec: "0 5 * * *",       // push 24h report at 0500 UTC
  FrequentReportCronSpec: "0 */2 * * *",  // push foF2, etc every 2nd hour
  ScrapeCronSpec: "*/15 * * * *",         // scrape all ionograms every 15 minutes
  ScrapeTimeout: 15 * time.Second,        // http.Client timeout
}

var db *sql.DB

func openDB(dbfile string) (*sql.DB) {
  var d *sql.DB
  d, err := sql.Open("sqlite3", dbfile)
  if err != nil {
    log.Fatalf("Cannot open db file %s: %v", dbfile, err)
  }
  return d
}

type Ionosonde struct {
  IonosondeId string
  UrsiCode string
  Name string
  Latitude sql.NullFloat64
  Longitude sql.NullFloat64
  ImageUrl string
  Filter sql.NullString
  DateFormat string
  DateCrop string
  Fof2Crop sql.NullString
  Fof1Crop sql.NullString
  FoeCrop sql.NullString
  FxiCrop sql.NullString
  FoesCrop sql.NullString
  FminCrop sql.NullString
  Hmf2Crop sql.NullString
  HmeCrop sql.NullString
  Push sql.NullBool
  Enabled sql.NullBool
}

type Parameters struct {
  ParameterId string
  IonosondeId string
  Date time.Time
  FoF2 sql.NullFloat64
  FoF1 sql.NullFloat64
  FoE sql.NullFloat64
  FxI sql.NullFloat64
  FoEs sql.NullFloat64
  Fmin sql.NullFloat64
  HmF2 sql.NullFloat64
  HmE sql.NullFloat64
}

type DailyReportParams struct {
  Hour string
  FoF2 sql.NullFloat64
  FoE sql.NullFloat64
  Fmin sql.NullFloat64
  QSOQRG sql.NullFloat64
  HmF2 sql.NullFloat64
  HmE sql.NullFloat64
  HamBands string
}


// getText from image
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
func getTextFromCut(img image.Image, xywh string) (string, error) {
  n := []int{}
  xywh = strings.TrimSpace(xywh)
  xywhU := strings.ToUpper(xywh)
  if len(xywh) == 0 || strings.HasPrefix(xywhU, "NA") ||
      strings.HasPrefix(xywhU, "#") || strings.HasPrefix(xywhU, "-") {
    return "", nil
  }
  s := strings.Split(xywh, ",")
  if len(s) != 4 {
    return "", errors.New("Wrong bounding-box format for xywh")
  }
  for i := range s {
    txt := strings.TrimSpace(s[i])
    num, err := strconv.Atoi(txt)
    if err != nil {
      return "", fmt.Errorf("xywh format error: %s is not an integer", txt)
    }
    n = append(n, num)
  }
  crop, err := cutter.Crop(img, cutter.Config{
    Mode: cutter.TopLeft,
    Anchor: image.Point{n[0], n[1]},
    Width: n[2],
    Height: n[3],
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

func getTextFromCutFloat64(img image.Image, xywh string) (float64, error) {
  textPreSwab, err := getTextFromCut(img, xywh)
  if err != nil {
    return float64(0.0), err
  }
  reg := regexp.MustCompile(`[^0-9\.]+`)
  txt := reg.ReplaceAllString(textPreSwab, "")
  if len(txt) == 0 {
    return float64(0.0), errors.New("Not a number")
  }
  num, err := strconv.ParseFloat(txt, 32)
  if err != nil {
    return float64(0.0), err
  }
  return float64(num), nil
}

var outputs = map[string]string{}
func downloadFile(url string, tag string) (string, error) {
  tr := &http.Transport{
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
  }
  client := &http.Client{
    Transport: tr,
    Timeout: cnf.ScrapeTimeout,
  }
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
  reader, err := os.Open(out.Name())
  if err != nil {
    os.Remove(out.Name())
    return "", err
  }
  _, format, err := image.DecodeConfig(reader)
  reader.Close()
  if err != nil {
    os.Remove(out.Name())
    return "", err
  }
  newOutFile := out.Name() + "." + format
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
func cleanup() {
  for _, v := range outputs {
    os.Remove(v)
  }
  outputs = map[string]string{}
}


/* applyFilter will apply an image filter specified in the filter column in the
 * ionosonde table of the database.
 */
func applyFilter(src image.Image, filter, ursiCode string) (image.Image) {
  var g *gift.GIFT
  const (
    strInvert string = `invert`
    strGrayscale string = `grayscale`
    strBlackAndWhite string = `blackandwhite`
    strInvertAndGrayscale string = `invertandgrayscale`
    strInvertAndBlackAndWhite string = `invertandblackandwhite`
    applyTxt string = `Applying filter %s to %s ionogram`
  )
  f := strings.ToLower(strings.TrimSpace(filter))
  switch f {
    case "":
      fallthrough
    case "none":
      fallthrough
    case "na":
      fallthrough
    case "n/a":
      fallthrough
    case "nil":
      return src
    case strInvert:
      log.Infof(applyTxt, f, ursiCode)
      g = gift.New(
        gift.Invert(),
      )
      dst := image.NewRGBA(g.Bounds(src.Bounds()))
      g.Draw(dst, src)
      return dst
    case strGrayscale:
      log.Infof(applyTxt, f, ursiCode)
      g = gift.New(
        gift.Grayscale(),
      )
      dst := image.NewRGBA(g.Bounds(src.Bounds()))
      g.Draw(dst, src)
      return dst
    case strBlackAndWhite:
      log.Infof(applyTxt, f, ursiCode)
      g = gift.New(
        gift.Grayscale(),
        gift.Brightness(-40),
        gift.Contrast(80),
      )
      dst := image.NewRGBA(g.Bounds(src.Bounds()))
      g.Draw(dst, src)
      return dst
    case strInvertAndGrayscale:
      log.Infof(applyTxt, f, ursiCode)
      g = gift.New(
        gift.Invert(),
        gift.Grayscale(),
      )
      dst := image.NewRGBA(g.Bounds(src.Bounds()))
      g.Draw(dst, src)
      return dst
    case strInvertAndBlackAndWhite:
      log.Infof(applyTxt, f, ursiCode)
      g = gift.New(
        gift.Invert(),
        gift.Grayscale(),
        gift.Brightness(-40),
        gift.Contrast(80),
      )
      dst := image.NewRGBA(g.Bounds(src.Bounds()))
      g.Draw(dst, src)
      return dst
    default:
      log.Errorf("Unknown filter %s, skipping filter for %s ionogram", f, ursiCode)
  }
  return src
}


/* getIonosondesFromDb() is used by ionize() and the make*Report() functions.
 */
func getIonosondesFromDb(sqlsuffix string) ([]Ionosonde, error) {
  var ionosondes []Ionosonde
  rows, err := db.Query("select ionosondeId, ursiCode, name, latitude, longitude, " +
                        "imageUrl, filter, dateFormat, " +
                        "dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, " +
                        "foesCrop, fminCrop, hmf2Crop, hmeCrop " +
                        "from ionosondes " + sqlsuffix)
  if err != nil {
    log.Errorf("Database query failed, cannot populate ionogram parameters: %v", err)
    return ionosondes, err
  }
  defer rows.Close()
  // sqlite not in WAL mode does not support concurrent select and insert on
  // two different tables so we read all ionosonde properties into memory instead...
  for rows.Next() {
    ti := Ionosonde{}
    err = rows.Scan(&ti.IonosondeId, &ti.UrsiCode, &ti.Name, &ti.Latitude, &ti.Longitude,
                    &ti.ImageUrl, &ti.Filter, &ti.DateFormat,
                    &ti.DateCrop, &ti.Fof2Crop, &ti.Fof1Crop, &ti.FoeCrop, &ti.FxiCrop,
                    &ti.FoesCrop, &ti.FminCrop, &ti.Hmf2Crop, &ti.HmeCrop)
    if err != nil {
      log.Errorf("rows.Scan error: %v", err)
      return ionosondes, err
    }
    ionosondes = append(ionosondes, ti)
  }
  err = rows.Err()
  if err != nil {
    log.Errorf("rows.Next() error: %v", err)
    return ionosondes, err
  }
  return ionosondes, nil
}


/* fixDate(string) is used by ionize() to replace common misinterpretations of the date string by tesseract
 */
func fixDate(dt string) (string) {
  replaceslice := [][2]string{
    { "oOct", "Oct" },
    { "Hov", "Nov" },
    { " 610 ", " 010 " },
    { " 611 ", " 011 " },
    { " 612 ", " 012 " },
    { " 613 ", " 013 " },
    { " 614 ", " 014 " },
    { " 615 ", " 015 " },
    { " 616 ", " 016 " },
    { " 617 ", " 017 " },
    { " 618 ", " 018 " },
    { " 619 ", " 019 " },
    { " 620 ", " 020 " },
    { " 621 ", " 021 " },
    { " 622 ", " 022 " },
    { " 623 ", " 023 " },
    { " 624 ", " 024 " },
    { " 625 ", " 025 " },
    { " 626 ", " 026 " },
    { " 627 ", " 027 " },
    { " 628 ", " 028 " },
    { " 629 ", " 029 " },
    { " 630 ", " 030 " },
    { " 631 ", " 031 " },
    { " 632 ", " 032 " },
    { " 633 ", " 033 " },
    { " 634 ", " 034 " },
    { " 635 ", " 035 " },
    { " 636 ", " 036 " },
    { " 637 ", " 037 " },
    { " 638 ", " 038 " },
    { " 639 ", " 039 " },
    { " 640 ", " 040 " },
    { " 641 ", " 041 " },
    { " 642 ", " 042 " },
    { " 643 ", " 043 " },
    { " 644 ", " 044 " },
    { " 645 ", " 045 " },
    { " 646 ", " 046 " },
    { " 647 ", " 047 " },
    { " 648 ", " 048 " },
    { " 649 ", " 049 " },
    { " 650 ", " 050 " },
    { " 0620 ", " 020 " },
    { " 0622 ", " 022 " },
    { " 6011 ", " 011 " },
    { " 6012 ", " 012 " },
    { " 6013 ", " 013 " },
    { " 6014 ", " 014 " },
    { " 6015 ", " 015 " },
    { " 6016 ", " 016 " },
    { " 6017 ", " 017 " },
    { " 6018 ", " 018 " },
    { " 6019 ", " 019 " },
    { " 6020 ", " 020 " },
    { " 6021 ", " 021 " },
    { " 6022 ", " 022 " },
    { " 6023 ", " 023 " },
    { " 6024 ", " 024 " },
    { " 6025 ", " 025 " },
    { " 6026 ", " 026 " },
    { " 6027 ", " 027 " },
    { " 6028 ", " 028 " },
    { " 6029 ", " 029 " },
    { " 6030 ", " 030 " },
    { " 6031 ", " 031 " },
    { " 6032 ", " 032 " },
    { " 6033 ", " 033 " },
    { " 6034 ", " 034 " },
    { " 6035 ", " 035 " },
    { " 6036 ", " 036 " },
    { " 6037 ", " 037 " },
    { " 6038 ", " 038 " },
    { " 6039 ", " 039 " },
    { " 6040 ", " 040 " },
    { " 6041 ", " 041 " },
    { " 6042 ", " 042 " },
    { " 6043 ", " 043 " },
    { " 6044 ", " 044 " },
    { " 6045 ", " 045 " },
    { " 6046 ", " 046 " },
    { " 6047 ", " 047 " },
    { " 6048 ", " 048 " },
    { " 6049 ", " 049 " },
    { " 6050 ", " 050 " },
    { "Febl17 ", "Feb17 "},
    { "Febl?7 ", "Feb17 "},
    { "Febl1? ", "Feb17 "},
    { "Febl? ", "Feb17 "},
    { "Febl?7 ", "Feb17 "},
    { "Febl16 ", "Feb16 "},
    { "Feb1l5 ", "Feb15 "},
    { "Febl15 ", "Feb15 "},
    { "Febl14 ", "Feb14 "},
    { "Febl13 ", "Feb13 "},
    { "Febl1?2 ", "Feb12 "},
    { "Feb1? ", "Feb12 "},
    { "Febl1? ", "Feb12 "},
    { "Febl? ", "Feb12 "},
    { "Febl12 ", "Feb12 "},
    { "Febl? ", "Feb12 "},
    { "Feb1?2 ", "Feb12 "},
    { "Feb1l2 ", "Feb12 "},
    { "Feb1l3 ", "Feb13 "},
    { "Feb11?2 ", "Feb12 "},
    { "Feb0O7 ", "Feb07 "},
    { "Feb007 ", "Feb07 "},
    { "Feb@7 ", "Feb07 "},
    { "Feb@5 ", "Feb05 "},
    { "FebO05 ", "Feb05 "},
    { "FebO06 ", "Feb06 "},
    { "FebO? ", "Feb07 "},
    { "DecO01 ", "Dec01 "},
    { "DecO@1 ", "Dec01 "},
    { "DecO1l ", "Dec01 "},
    { "DecOl ", "Dec01 "},
    { "DecO?2 ", "Dec02 " },
    { "DecO@2 ", "Dec02 " },
    { "DecO? ", "Dec07 " },
    { "Dec1? ", "Dec12 " },
    { "Decl? ", "Dec18 " },
    { "Jan?21 ", "Jan21 " },
    { "JanlO ", "Jan10 " },
    { "JanlO0 ", "Jan10 " },
    { "Jan1O0 ", "Jan10 " },
    { "Jani0 ", "Jan10 " },
    { "Jan110 ", "Jan10 " },
    { "Jani0 ", "Jan10 " },
    { "Jan1O ", "Jan10 " },
    { "Jani10 ", "Jan10 "},
    { "Jan1l ", "Jan11 " },
    { "Janii1 ", "Jan11 " },
    { "Janil ", "Jan11 " },
    { "Jani? 017 ", "Jan17 017 " },
    { "Jani? ", "Jan12 " },
    { "Janl? ", "Jan12 " },
    { "Jani15 ", "Jan15 " },
    { "Jani16 ", "Jan16 " },
    { "Jani17 ", "Jan17 " },
    { "Jani18 ", "Jan18 " },
    { "Jani19 ", "Jan19 " },
    { "Janl10 ", "Jan10 " },
    { "Janl11 ", "Jan11 " },
    { "Janl12 ", "Jan12 " },
    { "Janl13 ", "Jan13 " },
    { "Janl14 ", "Jan14 " },
    { "Janl15 ", "Jan15 " },
    { "Janl16 ", "Jan16 " },
    { "Janl17 ", "Jan17 " },
    { "Janl18 ", "Jan18 " },
    { "Janl19 ", "Jan19 " },
    { "Jani3 ", "Jan13 " },
    { "Jani4 ", "Jan14 " },
    { "Jani5 ", "Jan15 " },
    { "Jani6 ", "Jan16 " },
    { "Jani7 ", "Jan17 " },
    { "Jani8 ", "Jan18 " },
    { "Jani9 ", "Jan19 " },
    { "Janil3 ", "Jan13 " },
    { "Janil4 ", "Jan14 " },
    { "Janil5 ", "Jan15 " },
    { "Janil6 ", "Jan16 " },
    { "Janil7 ", "Jan17 " },
    { "Janil8 ", "Jan18 " },
    { "Janil9 ", "Jan19 " },
    { "Jani13 ", "Jan13 " },
    { "Jan1l5 ", "Jan15 " },
    { "Jan0O", "Jan0" },
    { "JanO0", "Jan0" },
    { "JanO", "Jan0" },
    { "Decl10 ", "Dec10 " },
    { "Decl11 ", "Dec11 " },
    { "Decl12 ", "Dec12 " },
    { "Decl13 ", "Dec13 " },
    { "Decl14 ", "Dec14 " },
    { "Decl15 ", "Dec15 " },
    { "Decl16 ", "Dec16 " },
    { "Decl17 ", "Dec17 " },
    { "Decl18 ", "Dec18 " },
    { "Decl19 ", "Dec19 " },
    { "DeclO ", "Dec10 " },
    { "Decll ", "Dec11 " },
    { "110 ", "10 " },
    { "1O@ ", "10 " },
    { "10O ", "10 " },
    { "1O ", "10 " },
    { " O0", " 00" },
    { "Jan00", "Jan0" },
    { "Feb00", "Feb0" },
    { "Mar00", "Mar0" },
    { "Apr00", "Apr0" },
    { "May00", "May0" },
    { "Jun00", "Jun0" },
    { "Jul00", "Jul0" },
    { "Aug00", "Aug0" },
    { "Sep00", "Sep0" },
    { "Oct00", "Oct0" },
    { "Nov00", "Nov0" },
    { "Dec00", "Dec0" },
    { "Dec@", "Dec0" },
    { "Novl19 ", "Nov19 " },
    { "Nov1?7 ", "Nov17 " },
    { "Nov1? ", "Nov17 " },
    { "Nov1l5", "Nov15" },
    { "Nov1?2", "Nov12" },
    { "Novl16", "Nov16" },
    { "Nov113 ", "Nov13 " },
    { "Nov114 ", "Nov14 " },
    { "Nov115 ", "Nov15 " },
    { "Nov116 ", "Nov16 " },
    { "Nov117 ", "Nov17 " },
    { "Nov118 ", "Nov18 " },
    { "Nov119 ", "Nov19 " },
    { "NovO01l ", "Nov01 " },
    { "NovO0l1 ", "Nov01 " },
    { "NovO1l ", "Nov01 " },
    { "NovO01 ", "Nov01 " },
    { "NovO1 ", "Nov01 " },
    { "Nov@l ", "Nov01 " },
    { "NovOl ", "Nov01 " },
    { "NovO0", "Nov0" },
    { "Nov@", "Nov0" },
    { "NovO", "Nov0" },
    { "Novl", "Nov1" },
    { "Decl", "Dec1" },
    { "Janl", "Jan1" },
    { "Febl", "Feb1" },
    { "Marl", "Mar1" },
    { "Aprl", "Apr1" },
    { "Junl", "Jun1" },
    { "Jull", "Jul1" },
    { "Augl", "Aug1" },
    { "Octl", "Oct1" },
    { "O1 ", "01 " },
    { "O2 ", "02 " },
    { "O3 ", "03 " },
    { "O4 ", "04 " },
    { "O5 ", "05 " },
    { "O6 ", "06 " },
    { "O7 ", "07 " },
    { "O8 ", "08 " },
    { "O9 ", "09 " },
  }
  for _, v := range replaceslice {
    dt = strings.Replace(dt, v[0], v[1], 1)
  }
  return dt
}

/* ionize() runs through ionosondes in db, downloads ionograms and populates
 * the parameters table in the database.
 */
func ionize() (error) {
  // TODO: prepare insert into parameters - statement before for loop starts

  rand.Seed(time.Now().UnixNano())
  r := rand.Intn(30)
  log.Infof("Scraping ionograms in %s", time.Duration(r) * time.Second)
  time.Sleep(time.Duration(r) * time.Second)

  mu.Lock()
  defer mu.Unlock()

  ionosondes, err := getIonosondesFromDb("where scrape=1")
  if err != nil {
    return err
  }

  for _, i := range ionosondes {
    // run an anonymous function inside this loop to be able to use defer
    func() {
      p := Parameters{}
      skipmsg := fmt.Sprintf("Skipping scrape of ionosonde %s (%s)", i.UrsiCode, i.Name)
      p.IonosondeId = i.IonosondeId

      log.Infof("Scraping %s (%s)", i.UrsiCode, i.Name)

      // download ionogram
      urls := strings.Split(i.ImageUrl, `,`)
      var imgFile string
      var url string
      var err error
      for z := range urls {
        imgFile, err = downloadFile(urls[z], i.UrsiCode)
        if err == nil {
          url = urls[z]
          break
        }
      }
      if err != nil {
        log.Errorf("Error downloading %v: %v", urls, err)
        log.Warning(skipmsg)
        return
      }
      defer cleanup()

      // open and decode downloaded image
      reader, err := os.Open(imgFile)
      if err != nil {
        log.Errorf("Cannot open ionogram %s: %v", imgFile, err)
        log.Warning(skipmsg)
        return
      }
      defer reader.Close()
      img, _, err := image.Decode(reader)
      if err != nil {
        log.Errorf("Cannot decode ionogram %s: %v", imgFile, err)
        log.Warning(skipmsg)
        return
      }

      // apply filter (if any specified) to img object
      if i.Filter.Valid {
        // applyFilter() will return the same img object if filter is empty,
        // none, nil, etc...
        img = applyFilter(img, i.Filter.String, i.UrsiCode)
/** for debug purposes:
        f, err := os.Create(i.UrsiCode + ".png")
        if err != nil {
          log.Errorf("Cannot create file: %v", err)
        } else {
          defer f.Close()
          err = png.Encode(f, img)
          if err != nil {
            log.Errorf("Cannot encode png: %v", err)
          }
        }
*/
      }

      // getTextFromCut
      // first get date
      ocrdt, err := getTextFromCut(img, i.DateCrop)
      if err != nil {
        log.Errorf("Cannot read date from ionogram %s: %v", imgFile, err)
        log.Warning(skipmsg)
        return
      }
      // fix common misinterpretations of the date string
      dt := fixDate(ocrdt)
      if dt != ocrdt {
        log.Infof("fixDate() changed '%s' to '%s'", ocrdt, dt)
      }
      // parse fixed date into time.Time
      p.Date, err = time.Parse(i.DateFormat, dt)
      if err != nil {
        log.Errorf("Cannot parse '%s' (according to format %s) from %s: %v", dt, i.DateFormat, imgFile, err)
        log.Warning(skipmsg)
        return
      }
      // populate parameters struct, as they are all float64 we can loop through them.
      // the indexes of these slice pairs need to match exactly...
      // QRG = frequency, to omit invalid values (only accept valus betweeen
      // 0.5 and 19.0 MHz)
      irQRG := []*sql.NullString{ &i.Fof2Crop, &i.Fof1Crop, &i.FoeCrop, &i.FxiCrop,
                                &i.FoesCrop, &i.FminCrop }
      prQRG := []*sql.NullFloat64{ &p.FoF2, &p.FoF1, &p.FoE, &p.FxI, &p.FoEs, &p.Fmin  }
      // QAH = elevation, to omit invalid ionosphere height (only accept values
      // beetween 60.0 and 999.0 km)
      irQAH := []*sql.NullString{ &i.Hmf2Crop, &i.HmeCrop }
      prQAH := []*sql.NullFloat64{ &p.HmF2, &p.HmE }

      for x := range irQRG {
        if irQRG[x].Valid {
          v, err := getTextFromCutFloat64(img, irQRG[x].String)
          if err == nil {
            if v >= 0.5 && v <= 19.0 {
              prQRG[x].Float64 = v
              prQRG[x].Valid = true
            } else {
              log.Warningf("Invalid frequency on %s ionogram, skipping: %f", i.UrsiCode, v)
            }
          }
          // bool is false by default, so Valid will be false if not set
        }
      }
      for x := range irQAH {
        if irQAH[x].Valid {
          v, err := getTextFromCutFloat64(img, irQAH[x].String)
          if err == nil {
            if v >= 60.0 && v <= 999.0 {
              prQAH[x].Float64 = v
              prQAH[x].Valid = true
            } else {
              log.Warningf("Invalid height on %s ionogram, skipping: %f", i.UrsiCode, v)
            }
          }
        }
      }

      // populate the parameters table in the database, but first...
      // check if we already have this metric...
      var countStr string
      err = db.QueryRow(fmt.Sprintf("select count(*) from parameters " +
                        "where ionosondeId=%s and dt='%s'",
                        i.IonosondeId, p.Date.Format(SqliteDateFormat))).Scan(&countStr)
      if err != nil {
        log.Errorf("QueryRow failed: %v", err)
        log.Warning(skipmsg)
        return
      }
      count, err := strconv.Atoi(countStr)
      if err != nil {
        log.Errorf("strconv.Atoi() failed: %v", err)
        log.Warning(skipmsg)
        return
      }
      if count > 0 {
        log.Warningf("Skipping parameters from %s with time %s, already in database", i.UrsiCode, p.Date.Format(i.DateFormat))
        return
      }
      // insert into parameters table...
      _, err = db.Exec("insert into parameters (ionosondeId, " +
          "dt, fof2, fof1, foe, fxi, foes, fmin, hme, hmf2) " +
          "values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
          i.IonosondeId, p.Date.Format(SqliteDateFormat), p.FoF2, p.FoF1,
          p.FoE, p.FxI, p.FoEs, p.Fmin, p.HmE, p.HmF2)
      if err != nil {
        log.Errorf("Unable to insert ionogram data into parameters table: %v", err)
        log.Warning(skipmsg)
        return
      }
      log.Infof("Scraped %s (%s) ionogram %s from %s", i.UrsiCode, i.Name, p.Date.Format(i.DateFormat), url)
    }()
  }
  return nil
}





/* makeDailyReports() is used by pushDailyReports() to make a text table of foF2
 * and other parameters with hourly averages over the last 24 hours.
 */
func makeDailyReports() ([]string, error) {
  type reportStruct struct {
    sunrise time.Time
    sunset time.Time
    solarNoon time.Time
    tag string
    fof2 string
    nvisRange string
    foe string
    fmin string
    hmf2 string
    hme string
    hamBands string
    low float64
    qsoqrg float64
  }
  const (
    notAvailable string = `NA`
    reportHeader string = "HH fmin  foF2  NVIS range  hmF2 HamBands\n"
    reportRow string = "%s%s%s %s %s %s %s\n"
  )
  var out []string
  log.Info("Producing 24h reports")
  ionosondes, err := getIonosondesFromDb("where enabled=1")
  if err != nil {
    return out, err
  }
  mu.Lock()
  defer mu.Unlock()
  for _, i := range ionosondes {
    func() {
      sunriseHour := ""
      noonHour := ""
      sunsetHour := ""
      r := fmt.Sprintf("24H %s (%s) DTG %s\n",
                      i.UrsiCode, i.Name, time.Now().UTC().Format(DTGFormat))
      if i.Latitude.Valid && i.Longitude.Valid {
        now := time.Now()
        times := suncalc.GetTimes(now, i.Latitude.Float64, i.Longitude.Float64)
        sunrise := times[suncalc.Sunrise].Time.UTC().Format(HourMinute)
        // add 15 minutes in case e.g 1045, hour would be 11 instead of 10, better
        sunriseHour = times[suncalc.Sunrise].Time.Add(15 * time.Minute).UTC().Format(Hour)
        noon := times[suncalc.SolarNoon].Time.UTC().Format(HourMinute)
        // add 30 min to solar noon to average it better (1030 would be 11, 1025 would be 10, etc)
        noonHour = times[suncalc.SolarNoon].Time.Add(30 * time.Minute).UTC().Format(Hour)
        sunset := times[suncalc.Sunset].Time.UTC().Format(HourMinute)
        // if sunset is 1055, it's better that sunset hour is 10 not to miss the sunset :), 1015 would still be hour 10, etc
        sunsetHour = times[suncalc.Sunset].Time.UTC().Format(Hour)
        r += fmt.Sprintf("+=sunrise=%s *=noon=%s -=sunset=%s\n", sunrise, noon, sunset)
      } else {
        r += "WARNING: No coordinates available!\n"
      }
      r += "NVIS range is fmin or foE to foF2*0.85\n"
      rows, err := db.Query(
        "select strftime('%H', dt), avg(fof2), avg(fof2)*0.85, avg(foe), avg(fmin), " +
        "avg(hmf2), avg(hme) from parameters where ionosondeId=? and " +
        "dt >= datetime('now','-1 days') and dt < datetime('now') " +
        "group by strftime('%H', dt) order by dt", i.IonosondeId)
      if err != nil {
        log.Errorf("Database query failed, cannot produce report for %s ionosonde: %v", i.UrsiCode, err)
        return
      }
      defer rows.Close()
      // add header
      r += reportHeader
      for rows.Next() {
        frp := DailyReportParams{}
        err = rows.Scan(&frp.Hour, &frp.FoF2, &frp.QSOQRG, &frp.FoE, &frp.Fmin,
                        &frp.HmF2, &frp.HmE)
        if err != nil {
          log.Errorf("rows.Scan error: %v", err)
          return
        }

        // format the values as strings before parsing them into the report
        rs := reportStruct{}

        // populate rs struct, first figure out if hour is sunrise, sunset or solar noon...
        rs.tag = " "
        if frp.Hour == sunriseHour {
          rs.tag = "+"
        }
        if frp.Hour == sunsetHour {
          rs.tag = "-"
        }
        if frp.Hour == sunriseHour && frp.Hour == sunsetHour {
          rs.tag = "±"
        }
        // solar noon has precedence
        if frp.Hour == noonHour {
          rs.tag = "*"
        }
        rs.fmin = fmt.Sprintf("%-5s", notAvailable)
        rs.fof2 = fmt.Sprintf("%-5s", notAvailable)
        rs.nvisRange = fmt.Sprintf("%-11s", notAvailable)
        rs.foe = fmt.Sprintf("%-5s", notAvailable)
        rs.hmf2 = fmt.Sprintf("%-4s", notAvailable)
        rs.hme = fmt.Sprintf("%-4s", notAvailable)
        rs.hamBands = notAvailable
        if frp.Fmin.Valid {
          rs.fmin = fmt.Sprintf("%-5.2f", frp.Fmin.Float64)
        }
        if frp.QSOQRG.Valid {
          rs.qsoqrg = frp.QSOQRG.Float64
          rs.low = frp.QSOQRG.Float64
        }
        if frp.FoF2.Valid {
          rs.fof2 = fmt.Sprintf("%-5.2f", frp.FoF2.Float64)
          if frp.FoE.Valid && frp.FoE.Float64 < frp.QSOQRG.Float64 {
            rs.nvisRange = fmt.Sprintf("%-11s", fmt.Sprintf("%.2f-%.2f", frp.FoE.Float64, frp.QSOQRG.Float64))
            rs.low = frp.FoE.Float64
          } else if frp.Fmin.Valid && frp.Fmin.Float64 < frp.QSOQRG.Float64 {
            rs.nvisRange = fmt.Sprintf("%-11s", fmt.Sprintf("%.2f-%.2f", frp.Fmin.Float64 , frp.QSOQRG.Float64))
            rs.low = frp.Fmin.Float64
          } else {
            rs.nvisRange = fmt.Sprintf("%-11s", fmt.Sprintf("?-%.2f", frp.QSOQRG.Float64))
          }
        }
        if frp.FoE.Valid {
          rs.foe = fmt.Sprintf("%-5.2f", frp.FoE.Float64)
        }
        if frp.HmF2.Valid {
          rs.hmf2 = fmt.Sprintf("%-4.0f", frp.HmF2.Float64)
        }
        if frp.HmE.Valid {
          rs.hme = fmt.Sprintf("%-4.0f", frp.HmE.Float64)
        }
        // redefine low as absolutely lowest reflected QRG
        if frp.Fmin.Valid {
          rs.low = frp.Fmin.Float64
        } else if frp.FoE.Valid {
          rs.low = frp.FoE.Float64
        } else {
          rs.low = rs.qsoqrg
        }
        // list usable ham bands
        hb := []string{}
        if rs.qsoqrg > 0 { // if rs.qsoqrg is above 0, so is rs.low
          if rs.low <= 2.0 && rs.qsoqrg >= 1.8 {
            hb = append(hb, "160")
          }
          if rs.low <= 3.8 && rs.qsoqrg >= 3.5 {
            hb = append(hb, "80")
          }
          if rs.low <= 5.3665 && rs.qsoqrg >= 5.3515 {
            hb = append(hb, "60")
          }
          if rs.low <= 7.2 && rs.qsoqrg >= 7.0 {
            hb = append(hb, "40")
          }
          if rs.low <= 10.15 && rs.qsoqrg >= 10.1 {
            hb = append(hb, "30")
          }
          if len(hb) > 0 {
            rs.hamBands = strings.Join(hb, ",")
          }
        }
        // output formatted row with parameters...
        // reportRow has 7 fields
        r += fmt.Sprintf(reportRow, frp.Hour, rs.tag, rs.fmin,
                  rs.fof2, rs.nvisRange, rs.hmf2, rs.hamBands)
      }
      // here we have a complete report (in the r var) for this ionosonde
      // append report to output
      out = append(out, r)
    }()
  }
  return out, nil
}

/* pushDailyReports() pushes the report created by makeDailyReports() to a
 * configured Discord integration URL (cnf.DiscordDailyWebhookUrl).
 */
func pushDailyReports() (error) {
  if ! cnf.Daily {
    log.Warningf("Option DAILY is false, will not push daily reports!")
    return nil
  }

  reports, err := makeDailyReports()

  if err != nil {
    log.Errorf("Unable to makeDailyReports(): %v", err)
    return err
  }
  pluralSuffix := ""
  if len(reports) > 0 { pluralSuffix = "s" }
  if cnf.Discord && ! cnf.Slack {
    log.Infof("Posting daily report%s to Discord", pluralSuffix)
  } else if ! cnf.Discord && cnf.Slack {
    log.Infof("Posting daily report%s to Slack", pluralSuffix)
  } else if cnf.Discord && cnf.Slack {
    log.Infof("Posting daily report%s to Slack and Discord", pluralSuffix)
  }
  for i := range reports {
    report := "```\n" + reports[i] + "\n```\n"
    if cnf.Discord {
      err := irmsg.SendDiscordMsg(cnf.DiscordDailyWebhookUrl, report)
      if err != nil {
        log.Errorf("Unable to post message to Discord: %v", err)
      }
    }
    if cnf.Slack {
      err := irmsg.SendSlackMsg(cnf.SlackDailyWebhookUrl, "24H report", report)
      if err != nil {
        log.Errorf("Unable to post message to Slack: %v", err)
      }
    }
    time.Sleep(5 * time.Second)
  }
  return nil
}




/* I have kept the linear regression prediction code commented out for later
 * use. This will be repurposed in a future feature release where the daily
 * report will also contain a prediction for the next 24 hours.
 *
type Point struct {
  X float64
  Y float64
}
func linearRegressionLSEnextVal(series []Point, nextX float64) float64 {
  // inspired by https://stackoverflow.com/a/16423799
  q := len(series)
  if q == 0 {
    return 0
  }
  p := float64(q)
  sum_x, sum_y, sum_xx, sum_xy := 0.0, 0.0, 0.0, 0.0
  for _, p := range series {
    sum_x += p.X
    sum_y += p.Y
    sum_xx += p.X * p.X
    sum_xy += p.X * p.Y
  }
  m := (p*sum_xy - sum_x*sum_y) / (p*sum_xx - sum_x*sum_x)
  b := (sum_y / p) - (m * sum_x / p)
  return nextX * m + b
}

func makePredictions(ips []IonogramParameters) ([]IonogramParameters) {
  var foF2s []Point
  var foEs []Point
  var fxIs []Point
  var predictions []IonogramParameters
  for _, ip := range ips {
    if ip.FoF2 >= 1 {
      foF2s = append(foF2s, Point{ X: float64(len(foF2s)+1), Y: float64(ip.FoF2),})
    }
    if ip.FoE >= 1 {
      foEs = append(foEs, Point{ X: float64(len(foEs)+1), Y: float64(ip.FoE),})
    }
    if ip.FxI >= 1 {
      fxIs = append(fxIs, Point{ X: float64(len(fxIs)+1), Y: float64(ip.FxI),})
    }
  }
  log.Infof("foF2 predictions: %v", foF2s)
  log.Infof("fxI predictions: %v", fxIs)
  for i := 1; i < cnf.Predictions+1 ; i++ {
    var pip IonogramParameters
    pip.FoF2 = float64( linearRegressionLSEnextVal(foF2s, float64(len(foF2s)+i)) )
    pip.FoE = float64( linearRegressionLSEnextVal(foEs, float64(len(foEs)+i)) )
    pip.FxI = float64( linearRegressionLSEnextVal(fxIs, float64(len(fxIs)+i)) )
    predictions = append(predictions, pip)
  }
  return predictions
}
*/




/* https://stackoverflow.com/a/40502637 */
type UTCFormatter struct {
  log.Formatter
}
func (u UTCFormatter) Format(e *log.Entry) ([]byte, error) {
  e.Time = e.Time.UTC()
  return u.Formatter.Format(e)
}

/* main() */
func main() {
/**** Keep this if json logging is not very popular...
  log.SetFormatter(UTCFormatter{&log.TextFormatter{
    FullTimestamp: true,
  }})
*/
  log.SetFormatter(UTCFormatter{&log.JSONFormatter{
    FieldMap: log.FieldMap{
      log.FieldKeyTime: "timestamp",
      log.FieldKeyLevel: "level",
      log.FieldKeyMsg: "message",
    },
  }})
  log.SetOutput(os.Stdout)
  log.SetLevel(log.InfoLevel)

  err := envconfig.Process("", cnf)
  if err != nil {
    log.Fatalf("envconfig.Process failed: %v", err)
  }

  if cnf.Discord {
    if cnf.Daily && cnf.DiscordDailyWebhookUrl == "" {
      log.Fatalf("Discord webhook URL for daily reports is not configured, configure with environment variable DAILY_DISCORDURL")
    }
    if cnf.Frequent && cnf.DiscordFrequentWebhookUrl == "" {
      log.Fatalf("Discord webhook URL for frequent reports is not configured, configure with environment variable FREQUENT_DISCORDURL")
    }
  }
  if cnf.Slack {
    if cnf.Daily && cnf.SlackDailyWebhookUrl == "" {
      log.Fatalf("Slack webhook URL for daily reports is not configured, configure with environment variable DAILY_SLACKURL")
    }
    if cnf.Frequent && cnf.SlackFrequentWebhookUrl == "" {
      log.Fatalf("Slack webhook URL for frequent reports is not configured, configure with environment variable FREQUENT_SLACKURL")
    }
  }

  if ( ! cnf.Daily ) && ( ! cnf.Frequent ) {
    log.Warning("Both daily and frequent reports are turned off, will only scrape ionograms and populate database. Enable daily or frequent reports to Slack or Discord with environment variable DAILY=true and/or FREQUENT=true")
  }

  if _, err := os.Stat(cnf.DatabaseFile); err == nil {
    // db file exists, just open it...
    db = openDB(cnf.DatabaseFile)
    defer db.Close()
  } else if os.IsNotExist(err) {
    // db file does not exist, initialize it...
    log.Infof("Creating and initializing database %s", cnf.DatabaseFile)
    f, err := os.OpenFile(cnf.DatabaseFile, os.O_CREATE, 0644)
    if err != nil {
      log.Fatalf("Cannot create db file %s: %v", cnf.DatabaseFile, err)
    }
    f.Close()
    db = openDB(cnf.DatabaseFile)
    defer db.Close()
    err = ionizedb.InitDB(db)
    if err != nil {
      log.Fatalf("Unable to initialize database: %v", err)
    }
    log.Infof("Database %s initialized successfully", cnf.DatabaseFile)
  } else {
    log.Fatalf("Cannot stat db file %s: %v", cnf.DatabaseFile, err)
  }

  log.Infof("Starting ionoreporter %s with db %s", version, cnf.DatabaseFile)

  c := cron.New(cron.WithLocation(time.UTC))
  log.Infof("Scheduling scrape function with cronspec %s", cnf.ScrapeCronSpec)
  _, err = c.AddFunc(cnf.ScrapeCronSpec, func(){ ionize() })
  if err != nil {
    log.Fatalf("Unable to schedule ionogram scrape function: %v", err)
  }

  if cnf.Discord || cnf.Slack {
    if cnf.Daily {
      log.Infof("Scheduling Slack and/or Discord daily reports with cronspec %s", cnf.DailyReportCronSpec)
      _, err = c.AddFunc(cnf.DailyReportCronSpec, func(){ pushDailyReports() })
      if err != nil {
        log.Fatalf("Unable to schedule full report function: %v", err)
      }
    }
/**** future feature...
    if cnf.Frequent {
      log.Infof("Scheduling Slack and/or Discord frequent reports with cronspec %s", cnf.FrequentReportCronSpec)
      _, err = c.AddFunc(cnf.FrequentReportCronSpec, func(){ pushFrequentReports() })
      if err != nil {
        log.Fatalf("Unable to schedule current report function: %v", err)
      }
    }
*/
  }

  c.Start()

/*** left for testing purposes...
  ionize()
  texts, err := makeDailyReports()
  if err != nil {
    log.Errorf("makeDailyReports(): %v", err)
  }
  for i := range texts {
    fmt.Printf("\n\n\n")
    fmt.Println(texts[i])
  }
*/
/*
  err = pushDailyReports()
  if err != nil {
    log.Errorf("%v", err)
  }
  return
*/

  log.Infof("ionoreporter started successfully")
  select{}
}
