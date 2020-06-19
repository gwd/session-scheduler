CREATE TABLE event_users(
    userid          text primary key,
    hashedpassword text not null,
    username        text not null unique,
    isadmin         boolean not null,
    isverified      boolean not null,
    realname        text,
    email           text,
    company         text,
    description     text,
    location        text not null /* Parsable by time.LoadLocation() */);

CREATE TABLE event_interest(
    userid text not null,
    discussionid text not null,
    interest integer not null,
    foreign key(userid) references event_users(userid),
    foreign key(discussionid) references event_discussions(discussionid),
    unique(userid, discussionid));

CREATE TABLE event_discussions(
    discussionid        text primary key,
    owner               text not null, /* FIXME: Would be better as ownerid */
    title               text not null,
    description         text,
    approvedtitle       text,
    approveddescription text,
    ispublic            boolean not null,
    foreign key(owner) references event_users(userid),
    unique(title));

CREATE TABLE event_discussions_possible_slots(
    discussionid text not null,
    slotid       text not null,
    foreign key(discussionid) references event_discussions(discussionid),
    foreign key(slotid) references event_slots(slotid),
    unique(discussionid, slotid));

/* Location ids should be in order and contiguous, starting at 1 */
CREATE TABLE event_locations(
    locationid          integer primary key,
    locationname        text not null,
    locationdescription text not null,
    isplace             boolean not null,
    capacity            integer not null);

/* Day names should be in order and contiguous, starting at 1 */
CREATE TABLE event_days(
    dayid integer primary key,
    dayname  text not null);

/* Every day should have associated slots with a slotidx's
 * in order and contiguous, starting at 1 */
CREATE TABLE event_slots(
    slotid   text primary key,
    slotidx  integer not null, /* Order within a day */
    dayid    integer not null,
    slottime string not null,  /* Output of time.MarshalText() */
    isbreak  boolean not null,
    islocked boolean not null,
    foreign  key(dayid) references event_days(dayid),
    unique(dayid, slotidx));

CREATE TABLE event_schedule(
    discussionid text not null,
    slotid       text not null,
    locationid   integer not null,
    foreign key(discussionid) references event_discussions(discussionid),
    foreign key(slotid) references event_slots(slotid),
    foreign key(locationid) references event_locations(locationid),
    unique(slotid, locationid));

