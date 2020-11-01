-- This patch will upgrade the database released with version 3.0.0 to support
-- version 3.1.0 with added columns latitude and longitude to the ionosondes
-- table. It will also add several new ionosondes.
--
-- See below how to enable/disable an ionogram from the reports or from being
-- scraped altogether.
--
-- You will need sqlite3 to upgrade, example:
-- sqlite3 ionoreporter.db < upgrade-db-from-300-to-310.sql

-- PRAGMA foreign_keys=off;
begin transaction;

create table new_ionosondes (
  ionosondeId integer primary key autoincrement,
  ursiCode varchar(16) not null,
  name varchar(64) not null,
  latitude float null,
  longitude float null,
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

insert into new_ionosondes(ionosondeId, ursiCode, name,
  imageUrl, filter, dateFormat,
  dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop,
  fminCrop, hmf2Crop, hmeCrop, scrape, enabled)
  select ionosondeId, ursiCode, name, imageUrl, filter, dateFormat,
  dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop,
  fminCrop, hmf2Crop, hmeCrop, scrape, enabled from ionosondes;

update new_ionosondes set latitude=54.62863, longitude=13.37433 where ursiCode="JR055";
update new_ionosondes set latitude=59.6, longitude=19.2 where ursiCode="TR169";
update new_ionosondes set latitude=37.9, longitude=284.5 where ursiCode="WP937";
update new_ionosondes set latitude=50.1, longitude=4.6 where ursiCode="DB049";
update new_ionosondes set latitude=41.8, longitude=12.5 where ursiCode="RA041";

drop table ionosondes;

create table ionosondes (
  ionosondeId integer primary key autoincrement,
  ursiCode varchar(16) not null,
  name varchar(64) not null,
  latitude float not null,
  longitude float not null,
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

insert into ionosondes(ionosondeId, ursiCode, name, latitude, longitude,
  imageUrl, filter, dateFormat,
  dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop,
  fminCrop, hmf2Crop, hmeCrop, scrape, enabled)
  select ionosondeId, ursiCode, name, latitude, longitude, imageUrl, filter,
  dateFormat, dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop,
  fminCrop, hmf2Crop, hmeCrop, scrape, enabled from new_ionosondes;

drop table new_ionosondes;

-- disable and do not scrape ionogram RA041, replace with RO041
update ionosondes set scrape=0, enabled=0 where ursiCode="RA041";
-- disable DB049 Dourbes, Chilton will replace it
update ionosondes set enabled=0 where ursiCode="DB049";

-- add ionogram EG931, THJ76, EB040, and RO041
insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "EG931",
    "Eglin AFB",
    30.5, 273.5,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=EG931",
    null,
    "2006 Jan02 002 150405",
    "325,30,195,17",
    "61,50,65,15",
    "61,66,65,15",
    "61,99,65,15",
    "61,130,65,15",
    "61,147,65,15",
    "61,162,65,15",
    "61,314,65,15",
    "61,346,65,15",
    1,
    1
);

insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "THJ76",
    "Thule",
    76.5, 291.6,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=THJ76",
    null,
    "2006 Jan02 002 150405",
    "323,30,197,17",
    "60,50,66,15",
    "60,67,66,15",
    "60,99,66,15",
    "60,130,66,15",
    "60,147,66,15",
    "60,162,66,15",
    "60,314,66,15",
    "60,346,66,15",
    1,
    0
);

insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "EB040",
    "Roquetes",
    40.8, 0.5,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=EB040",
    null,
    "2006 Jan02 002 150405",
    "323,30,197,17",
    "60,50,66,15",
    "60,67,66,15",
    "60,99,66,15",
    "60,130,66,15",
    "60,147,66,15",
    "60,162,66,15",
    "60,314,66,15",
    "60,346,66,15",
    1,
    1
);

insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "RO041",
    "Rome",
    41.9, 12.5,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=RO041",
    null,
    "2006 Jan02 002 150405",
    "323,30,197,17",
    "60,50,66,15",
    "60,67,66,15",
    "60,99,66,15",
    "60,130,66,15",
    "60,147,66,15",
    "60,162,66,15",
    "60,314,66,15",
    "60,346,66,15",
    1,
    0
);

insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "RL052",
    "Chilton",
    51.5, 359.4,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=RL052",
    null,
    "2006 Jan02 002 150405",
    "323,30,197,17",
    "60,50,66,15",
    "60,67,66,15",
    "60,99,66,15",
    "60,130,66,15",
    "60,147,66,15",
    "60,162,66,15",
    "60,314,66,15",
    "60,346,66,15",
    1,
    1
);

insert into ionosondes (ursiCode, name, latitude, longitude, imageUrl, filter, dateFormat,
    dateCrop, fof2Crop, fof1Crop, foeCrop, fxiCrop, foesCrop, fminCrop,
    hmf2Crop, hmeCrop, scrape, enabled)
  values (
    "FF051",
    "Fairford",
    51.7, 358.2,
    "https://lgdc.uml.edu/common/ShowRandomIonogram?ursiCode=FF051",
    null,
    "2006 Jan02 002 150405",
    "323,30,197,17",
    "60,50,66,15",
    "60,67,66,15",
    "60,99,66,15",
    "60,130,66,15",
    "60,147,66,15",
    "60,162,66,15",
    "60,314,66,15",
    "60,346,66,15",
    1,
    0
);

commit;
-- PRAGMA foreign_keys=on;

