create table attributes(
    key   text primay key,
    value text not null);
    /* "SchemaVersion" "1" */
    /* "SessionCookieName" "XenSummitWebSession" */
    /* "DefaultExpiry" itoa(24 * 3 * time.Hour) */

create table sessions(
    id       text primary key,
    userid   string not null,
    expiryts integer not null /* in Unix time */);
