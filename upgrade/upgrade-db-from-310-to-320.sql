-- This patch will upgrade the database released with version 3.1.0 to support
-- version 3.2.0 with added predictions table.
--
-- You will need sqlite3 to upgrade, example:
-- sqlite3 ionoreporter.db < upgrade-db-from-310-to-320.sql

-- PRAGMA foreign_keys=off;
begin transaction;

create table predictions (
  predictionId integer primary key not null,
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

commit;
-- PRAGMA foreign_keys=on;

