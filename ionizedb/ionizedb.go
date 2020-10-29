package ionizedb

import (
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
)

const createdbsql string = `
create table parameters (
  parameterId integer primary key not null,
  ionosondeId integer not null,
  dt datetime not null,
  fof2 float null,
  fof1 float null,
  foe float null,
  fxi float null,
  foes float null,
  fmin float null,
  hme float null,
  hmf2 float null
);

create table ionosondes (
  ionosondeId integer primary key autoincrement,
  ursiCode varchar(16) not null,
  name varchar(64) not null,
  imageUrl varchar(1024) not null,
  filter varchar(64) null,
  dateFormat varchar(32) not null,
  dateCrop varchar(20) not null,
  fof2Crop varchar(20) null,
  fof1Crop varchar(20) null,
  foeCrop varchar(20) null,
  fxiCrop varchar(20) null,
  foesCrop varchar(20) null,
  fminCrop varchar(20) null,
  hmf2Crop varchar(20) null,
  hmeCrop varchar(20) null,
  scrape boolean default 1,
  enabled boolean default 0
);

-- dateCrop, fof2Crop, etc are in the format of x,y,width,height
-- When selecting in Gimp, this will show as Position and Size in the
-- Rectagle Select property box

insert into ionosondes (ursiCode, name, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "JR055",
    "Juliusruh",
    "https://www.ionosonde.iap-kborn.de/LATEST.PNG,https://www.iap-kborn.de/fileadmin/user_upload/MAIN-abteilung/radar/Radars/Ionosonde/Plots/LATEST.PNG",
    null,
    "2006 Jan02 002 150405",
    "222,29,195,17",
    "36,50,90,15",
    "36,65,90,17",
    "27,98,101,16",
    "27,129,98,17",
    "36,145,90,17",
    "36,162,90,17",
    "37,313,91,17",
    "27,345,100,17",
    1,
    1
);

insert into ionosondes (ursiCode, name, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "TR169",
    "Tromso",
    "http://www.tgo.uit.no/ionosonde/latest.gif",
    null,
    "2006 Jan02 002 1504",
    "291,25,157,15",
    "37,52,73,15",
    "37,67,73,15",
    "37,97,73,15",
    "37,127,73,15",
    "37,142,73,15",
    "37,157,73,15",
    "37,298,73,15",
    "37,328,73,15",
    1,
    1
);

insert into ionosondes (ursiCode, name, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "WP937",
    "Wallops Is",
    "https://www.ngdc.noaa.gov/stp/IONO/rt-iono/latest/WP937.png",
    null,
    "2006 Jan02 002 150405",
    "270,30,177,17",
    "41,52,70,15",
    "41,68,70,15",
    "41,98,70,15",
    "41,128,70,15",
    "41,143,70,15",
    "41,158,70,15",
    "41,299,70,15",
    "41,329,70,15",
    1,
    0
);

insert into ionosondes (ursiCode, name, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "DB049",
    "Dourbes",
    "http://digisonde.oma.be/IonoGIF.secure/LATEST.PNG",
    null,
    "2006 Jan02 002 150405",
    "227,30,196,16",
    "45,50,82,15",
    "45,66,82,15",
    "45,98,82,15",
    "45,130,82,15",
    "45,146,82,15",
    "45,162,82,15",
    "45,314,82,15",
    "45,346,82,15",
    1,
    0
);

-- tesseract/gosseract cannot read parameters from the RA041 ionogram due to
-- the white-on-black ionogram style. The filter feature was created to be
-- able to invert the colors (to typical paper-like black on white) and also
-- increase/decrease brightness and contrast to make it black-and-white for
-- easier interpretation by tesseract. This seem to work.

insert into ionosondes (ursiCode, name, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "RA041",
    "Rome",
    "http://ionos.ingv.it/Roma/LATEST.GIF",
    "invertAndBlackAndWhite",
    "2006 01 02 - TIME (UT): 15:04",
    "309,0,185,16",
    "695,66,75,24",
    "695,189,75,24",
    "633,658,78,13",
    "695,158,75,24",
    "NA",
    "NA",
    "644,592,67,14",
    "644,671,67,14",
    1,
    0
);
`

func InitDB(db *sql.DB) (error) {
  _, err := db.Exec(createdbsql)
  return err
}
