CREATE TABLE event_users(
    userid      text primary key,
    bcryptpwd   text not null,
    username    text not null unique,
    isadmin     boolean not null,
    isverified  boolean not null,
    realname    text,
    email       text,
    company     text,
    description text);

CREATE TABLE event_interest(
    userid text not null,
    discussionid text not null,
    interest integer not null,
    foreign key(userid) references event_users(userid),
    foreign key(discussionid) references event_discussions(discussionid),
    unique(userid, discussionid));

CREATE TABLE event_discussions(
    discussionid  text primary key,
    userid        text not null,
    title         text not null,
    description   text,
    approvedtitle text,
    approveddesc  text,
    ispublic      boolean not null,
    foreign key(userid) references event_users(userid));

CREATE TABLE event_discussions_possible_slots(
    discussionid text not null,
    slotid       text not null,
    foreign key(discussionid) references event_discussions(discussionid),
    foreign key(slotid) references event_slots(slotid),
    unique(discussionid, slotid));

CREATE TABLE event_locations(
    locationid   text primary key,
    locationname text not null,
    isplace      boolean not null,
    capacity     integer not null);

CREATE TABLE event_days(
    dayid text primary key,
    dayname  text not null);

CREATE TABLE event_slots(
    slotid   text primary key,
    dayid    text not null,
    slottime string nto null,
    isbreak  boolean not null,
    islocked boolean not null,
    foreign  key(dayid) references event_days(dayid));

CREATE TABLE event_schedule(
    discussionid text not null,
    slotid       text not null,
    locationid   text not null,
    foreign key(discussionid) references event_discussions(discussionid),
    foreign key(slotid) references event_slots(slotid),
    foreign key(locationid) references event_slots(locationid),
    unique(discussionid, slotid, locationid));

